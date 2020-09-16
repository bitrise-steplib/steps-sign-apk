package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-steplib/steps-sign-apk/keystore"
	"github.com/bitrise-tools/go-android/sdk"
	"github.com/bitrise-tools/go-steputils/tools"
)

var signingFileExts = []string{".mf", ".rsa", ".dsa", ".ec", ".sf"}

// -----------------------
// --- Models
// -----------------------

type configs struct {
	BuildArtifactPath  string `env:"android_app,required"`
	KeystoreURL        string `env:"keystore_url,required"`
	KeystorePassword   string `env:"keystore_password,required"`
	KeystoreAlias      string `env:"keystore_alias,required"`
	PrivateKeyPassword string `env:"private_key_password"`
	OutputName         string `env:"output_name"`

	VerboseLog   bool   `env:"verbose_log,opt[true,false]"`
	PageAlign    string `env:"page_align,opt[automatic,true,false]"`
	SignerScheme string `env:"signer_scheme,opt[automatic,v2,v3,v4]"`

	// Deprecated
	APKPath string `env:"apk_path"`
}

type pageAlignStatus int

const (
	pageAlignInvalid pageAlignStatus = iota + 1
	pageAlignAuto
	pageAlignYes
	pageAlignNo
)

func parsePageAlign(s string) pageAlignStatus {
	switch s {
	case "automatic":
		return pageAlignAuto
	case "true":
		return pageAlignYes
	case "false":
		return pageAlignNo
	default:
		return pageAlignInvalid
	}
}

func splitElements(list []string, sep string) (s []string) {
	for _, e := range list {
		s = append(s, strings.Split(e, sep)...)
	}
	return
}

func parseAppList(list string) (apps []string) {
	list = strings.TrimSpace(list)
	if len(list) == 0 {
		return nil
	}

	s := []string{list}
	for _, sep := range []string{"\n", `\n`, "|"} {
		s = splitElements(s, sep)
	}

	for _, app := range s {
		app = strings.TrimSpace(app)
		if len(app) > 0 {
			apps = append(apps, app)
		}
	}
	return
}

// -----------------------
// --- Functions
// -----------------------

func download(url, pth string) error {
	out, err := os.Create(pth)
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			log.Warnf("Failed to close file: %s, error: %s", out, err)
		}
	}()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warnf("Failed to close response body, error: %s", err)
		}
	}()

	_, err = io.Copy(out, resp.Body)
	return err
}

func listFilesInBuildArtifact(aapt, pth string) ([]string, error) {
	cmdSlice := []string{aapt, "list", pth}
	out, err := keystore.ExecuteForOutput(cmdSlice)
	if err != nil {
		return []string{}, err
	}

	return strings.Split(out, "\n"), nil
}

func filterMETAFiles(fileList []string) []string {
	metaFiles := []string{}
	for _, file := range fileList {
		if strings.HasPrefix(file, "META-INF/") {
			metaFiles = append(metaFiles, file)
		}
	}
	return metaFiles
}

func filterSigningFiles(fileList []string) []string {
	signingFiles := []string{}
	for _, file := range fileList {
		ext := filepath.Ext(file)
		for _, signExt := range signingFileExts {
			if strings.EqualFold(ext, signExt) {
				signingFiles = append(signingFiles, file)
			}
		}
	}
	return signingFiles
}

func removeFilesFromBuildArtifact(aapt, pth string, files []string) error {
	cmdSlice := append([]string{aapt, "remove", pth}, files...)

	prinatableCmd := command.PrintableCommandArgs(false, cmdSlice)
	log.Printf("=> %s", prinatableCmd)

	out, err := keystore.ExecuteForOutput(cmdSlice)
	if err != nil && errorutil.IsExitStatusError(err) {
		return errors.New(out)
	}
	return err
}

func isBuildArtifactSigned(aapt, pth string) (bool, error) {
	filesInBuildArtifact, err := listFilesInBuildArtifact(aapt, pth)
	if err != nil {
		return false, err
	}

	metaFiles := filterMETAFiles(filesInBuildArtifact)

	for _, metaFile := range metaFiles {
		ext := filepath.Ext(metaFile)
		if strings.EqualFold(ext, ".dsa") || strings.EqualFold(ext, ".rsa") {
			return true, nil
		}
	}
	return false, nil
}

func unsignBuildArtifact(aapt, pth string) error {
	filesInBuildArtifact, err := listFilesInBuildArtifact(aapt, pth)
	if err != nil {
		return err
	}

	metaFiles := filterMETAFiles(filesInBuildArtifact)
	signingFiles := filterSigningFiles(metaFiles)

	if len(signingFiles) == 0 {
		log.Printf("Build Artifact is not signed")
		return nil
	}

	return removeFilesFromBuildArtifact(aapt, pth, signingFiles)
}

func zipalignBuildArtifact(zipalign, pth, dstPth string, pageAlign bool) error {
	cmdSlice := []string{zipalign}

	if pageAlign {
		cmdSlice = append(cmdSlice, "-p")
	}

	cmdSlice = append(cmdSlice, "-f", "4", pth, dstPth)
	prinatableCmd := command.PrintableCommandArgs(false, cmdSlice)
	log.Printf("=> %s", prinatableCmd)

	_, err := keystore.ExecuteForOutput(cmdSlice)
	return err
}

func prettyBuildArtifactBasename(buildArtifactPth string) string {
	buildArtifactBasenameWithExt := path.Base(buildArtifactPth)
	buildArtifactExt := filepath.Ext(buildArtifactBasenameWithExt)
	buildArtifactBasename := strings.TrimSuffix(buildArtifactBasenameWithExt, buildArtifactExt)
	buildArtifactBasename = strings.TrimSuffix(buildArtifactBasename, "-unsigned")
	return buildArtifactBasename
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v)
	os.Exit(1)
}

func handleDeprecatedInputs(cfg *configs) {
	if cfg.APKPath != "" {
		log.Warnf("step input 'APK file path' (apk_path) is deprecated and will be removed on 20 August 2019, use 'APK or App Bundle file path' (android_app) instead!")
		cfg.BuildArtifactPath = cfg.APKPath
	}
}

func validate(cfg configs) error {
	buildArtifactPaths := parseAppList(cfg.BuildArtifactPath)
	for _, buildArtifactPath := range buildArtifactPaths {
		if exist, err := pathutil.IsPathExists(buildArtifactPath); err != nil {
			return fmt.Errorf("failed to check if BuildArtifactPath exist at: %s, error: %s", buildArtifactPath, err)
		} else if !exist {
			return fmt.Errorf("BuildArtifactPath not exist at: %s", buildArtifactPath)
		}
	}
	return nil
}

// -----------------------
// --- Main
// -----------------------
func main() {
	var cfg configs
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Issue with input: %s", err)
	}
	pageAlignConfig := parsePageAlign(cfg.PageAlign)

	stepconf.Print(cfg)
	log.SetEnableDebugLog(cfg.VerboseLog)
	handleDeprecatedInputs(&cfg)
	fmt.Println()

	if err := validate(cfg); err != nil {
		failf("Issue with input: %s", err)
	}

	// Download keystore
	tmpDir, err := pathutil.NormalizedOSTempDirPath("bitrise-sign-build-artifact")
	if err != nil {
		failf("Failed to create tmp dir, error: %s", err)
	}

	keystorePath := ""
	if strings.HasPrefix(cfg.KeystoreURL, "file://") {
		pth := strings.TrimPrefix(cfg.KeystoreURL, "file://")
		var err error
		keystorePath, err = pathutil.AbsPath(pth)
		if err != nil {
			failf("Failed to expand path (%s), error: %s", pth, err)
		}
	} else {
		log.Infof("Download keystore")
		keystorePath = path.Join(tmpDir, "keystore.jks")
		if err := download(cfg.KeystoreURL, keystorePath); err != nil {
			failf("Failed to download keystore, error: %s", err)
		}
	}
	log.Printf("using keystore at: %s", keystorePath)

	keystore, err := keystore.NewHelper(keystorePath, cfg.KeystorePassword, cfg.KeystoreAlias)
	if err != nil {
		failf("Failed to create keystore helper, error: %s", err)
	}
	// ---

	// Find Android tools
	androidHome := os.Getenv("ANDROID_HOME")
	log.Printf("android_home: %s", androidHome)

	androidSDK, err := sdk.New(androidHome)
	if err != nil {
		failf("failed to create sdk model, error: %s", err)
	}

	aapt, err := androidSDK.LatestBuildToolPath("aapt")
	if err != nil {
		failf("Failed to find aapt path, error: %s", err)
	}
	log.Printf("aapt: %s", aapt)

	zipalign, err := androidSDK.LatestBuildToolPath("zipalign")
	if err != nil {
		failf("Failed to find zipalign path, error: %s", err)
	}
	log.Printf("zipalign: %s", zipalign)
	// ---

	// Sign build artifacts
	buildArtifactPaths := parseAppList(cfg.BuildArtifactPath)
	signedAPKPaths := make([]string, 0)
	signedAABPaths := make([]string, 0)

	log.Infof("signing %d Build Artifacts", len(buildArtifactPaths))
	fmt.Println()

	if len(buildArtifactPaths) > 1 && cfg.OutputName != "" {
		log.Warnf("output_name is set and more than one artifact found, disabling artifact renaming as it would result in overwriting exported artifacts")
		fmt.Println()
		cfg.OutputName = ""
	}

	for i, buildArtifactPath := range buildArtifactPaths {
		artifactExt := path.Ext(buildArtifactPath)
		log.Donef("%d/%d signing %s", i+1, len(buildArtifactPaths), buildArtifactPath)
		fmt.Println()

		buildArtifactDir := path.Dir(buildArtifactPath)
		buildArtifactBasename := prettyBuildArtifactBasename(buildArtifactPath)

		// unsign build artifact
		unsignedBuildArtifactPth := filepath.Join(tmpDir, "unsigned"+artifactExt)
		if err := command.CopyFile(buildArtifactPath, unsignedBuildArtifactPth); err != nil {
			failf("Failed to copy build artifact, error: %s", err)
		}

		isSigned, err := isBuildArtifactSigned(aapt, unsignedBuildArtifactPth)
		if err != nil {
			failf("Failed to check if build artifact is signed, error: %s", err)
		}

		if isSigned {
			log.Printf("Signature file (DSA or RSA) found in META-INF, unsigning the build artifact...")
			if err := unsignBuildArtifact(aapt, unsignedBuildArtifactPth); err != nil {
				failf("Failed to unsign Build Artifact, error: %s", err)
			}
			fmt.Println()
		} else {
			log.Printf("No signature file (DSA or RSA) found in META-INF, skipping build artifact unsign...")
			fmt.Println()
		}
		// ---

		// sign build artifact
		unalignedBuildArtifactPth := filepath.Join(tmpDir, "unaligned"+artifactExt)
		log.Infof("Sign Build Artifact")
		if err := keystore.SignBuildArtifact(unsignedBuildArtifactPth, unalignedBuildArtifactPth, cfg.PrivateKeyPassword); err != nil {
			failf("Failed to sign Build Artifact, error: %s", err)
		}
		fmt.Println()

		log.Infof("Verify Build Artifact")
		if err := keystore.VerifyBuildArtifact(unalignedBuildArtifactPth); err != nil {
			failf("Failed to verify Build Artifact, error: %s", err)
		}
		fmt.Println()

		log.Infof("Zipalign Build Artifact")
		signedArtifactName := fmt.Sprintf("%s-bitrise-signed%s", buildArtifactBasename, artifactExt)
		if artifactName := fmt.Sprintf("%s%s", cfg.OutputName, artifactExt); cfg.OutputName != "" {
			log.Printf("- Exporting (%s) as: %s", signedArtifactName, artifactName)
			signedArtifactName = artifactName
		}
		fullPath := filepath.Join(buildArtifactDir, signedArtifactName)

		if strings.EqualFold(artifactExt, ".aab") {
			signedAABPaths = append(signedAABPaths, fullPath)
		} else {
			signedAPKPaths = append(signedAPKPaths, fullPath)
		}

		pageAlign := pageAlignConfig == pageAlignYes
		// Only care about .so memory page alignment for APKs
		if !strings.EqualFold(artifactExt, ".aab") && pageAlignConfig == pageAlignAuto {
			extractNativeLibs, err := parseAPKextractNativeLibs(fullPath)
			if err != nil {
				log.Warnf("Failed to parse APK manifest to read extractNativeLibs attribute: %s", err)
				pageAlign = true
			} else {
				pageAlign = !extractNativeLibs
			}
		}

		if err := zipalignBuildArtifact(zipalign, unalignedBuildArtifactPth, fullPath, pageAlign); err != nil {
			failf("Failed to zipalign Build Artifact, error: %s", err)
		}
		fmt.Println()
		// ---
	}

	joinedAPKOutputPaths := strings.Join(signedAPKPaths, "|")
	joinedAABOutputPaths := strings.Join(signedAABPaths, "|")

	// APK
	if len(signedAPKPaths) > 0 {
		exportAPK(signedAPKPaths, joinedAPKOutputPaths)
	} else {
		log.Debugf("No Signed APK was exported - skip BITRISE_SIGNED_APK_PATH Environment Variable export")
		log.Debugf("No Signed APK was exported - skip BITRISE_SIGNED_APK_PATH_LIST Environment Variable export")
	}

	// AAB
	if len(signedAABPaths) > 0 {
		exportAAB(signedAABPaths, joinedAABOutputPaths)
	} else {
		log.Debugf("No Signed AAB was exported - skip BITRISE_SIGNED_AAB_PATH Environment Variable export")
		log.Debugf("No Signed AAB was exported - skip BITRISE_SIGNED_AAB_PATH_LIST Environment Variable export")
	}
}

func exportAPK(signedAPKPaths []string, joinedAPKOutputPaths string) {
	if err := tools.ExportEnvironmentWithEnvman("BITRISE_SIGNED_APK_PATH", signedAPKPaths[len(signedAPKPaths)-1]); err != nil {
		log.Warnf("Failed to export APK (%s) error: %s", signedAPKPaths[len(signedAPKPaths)-1], err)
	} else {
		log.Donef("The Signed APK path is now available in the Environment Variable: BITRISE_SIGNED_APK_PATH (value: %s)", signedAPKPaths[len(signedAPKPaths)-1])
	}

	if err := tools.ExportEnvironmentWithEnvman("BITRISE_SIGNED_APK_PATH_LIST", joinedAPKOutputPaths); err != nil {
		log.Warnf("Failed to export APK list (%s), error: %s", joinedAPKOutputPaths, err)
	} else {
		log.Donef("The Signed APK path list is now available in the Environment Variable: BITRISE_SIGNED_APK_PATH_LIST (value: %s)", joinedAPKOutputPaths)
	}

	if err := tools.ExportEnvironmentWithEnvman("BITRISE_APK_PATH", joinedAPKOutputPaths); err != nil {
		log.Warnf("Failed to export APK list (%s), error: %s", joinedAPKOutputPaths, err)
	} else {
		log.Donef("The Signed APK path is now available in the Environment Variable: BITRISE_APK_PATH (value: %s)", joinedAPKOutputPaths)
	}
}

func exportAAB(signedAABPaths []string, joinedAABOutputPaths string) {
	if err := tools.ExportEnvironmentWithEnvman("BITRISE_SIGNED_AAB_PATH", signedAABPaths[len(signedAABPaths)-1]); err != nil {
		log.Warnf("Failed to export AAB (%s), error: %s", signedAABPaths[len(signedAABPaths)-1], err)
	} else {
		log.Donef("The Signed AAB path is now available in the Environment Variable: BITRISE_SIGNED_AAB_PATH (value: %s)", signedAABPaths[len(signedAABPaths)-1])
	}

	if err := tools.ExportEnvironmentWithEnvman("BITRISE_SIGNED_AAB_PATH_LIST", joinedAABOutputPaths); err != nil {
		log.Warnf("Failed to export AAB list (%s), error: %s", joinedAABOutputPaths, err)
	} else {
		log.Donef("The Signed AAB path list is now available in the Environment Variable: BITRISE_SIGNED_AAB_PATH_LIST (value: %s)", joinedAABOutputPaths)
	}

	if err := tools.ExportEnvironmentWithEnvman("BITRISE_AAB_PATH", joinedAABOutputPaths); err != nil {
		log.Warnf("Failed to export AAB list (%s), error: %s", joinedAABOutputPaths, err)
	} else {
		log.Donef("The Signed AAB path is now available in the Environment Variable: BITRISE_AAB_PATH (value: %s)", joinedAABOutputPaths)
	}
}
