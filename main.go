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
	"unicode/utf8"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-steplib/steps-sign-apk/keystore"
	"github.com/bitrise-tools/go-android/sdk"
	"github.com/bitrise-tools/go-steputils/tools"
)

const jarsigner = "/usr/bin/jarsigner"

var signingFileExts = []string{".mf", ".rsa", ".dsa", ".ec", ".sf"}

// -----------------------
// --- Models
// -----------------------

// ConfigsModel ...
type ConfigsModel struct {
	BuildArtifactPath  string
	KeystoreURL        string
	KeystorePassword   string
	KeystoreAlias      string
	PrivateKeyPassword string
	JarsignerOptions   string
}

func createConfigsModelFromEnvs() ConfigsModel {
	cfg := ConfigsModel{
		KeystoreURL:        os.Getenv("keystore_url"),
		KeystorePassword:   os.Getenv("keystore_password"),
		KeystoreAlias:      os.Getenv("keystore_alias"),
		PrivateKeyPassword: os.Getenv("private_key_password"),
		JarsignerOptions:   os.Getenv("jarsigner_options"),
	}

	if val := os.Getenv("apk_path"); val != "" {
		log.Warnf("APK_PATH env detected. APK_PATH will be deprecated in future versions! Please use ANDROID_APP instead. Using APK_PATH value for current build.")
		cfg.BuildArtifactPath = val
		return cfg
	}

	if inputEnv := os.Getenv("android_app"); strings.Contains(inputEnv, "\n") {
		lines := strings.Split(inputEnv, "\n")

		if trimmed := strings.TrimSpace(lines[0]); trimmed != "" {
			cfg.BuildArtifactPath = trimmed
		} else {
			cfg.BuildArtifactPath = lines[1]
		}
	}

	return cfg
}

func (configs ConfigsModel) print() {
	fmt.Println()
	log.Infof("Configs:")
	log.Printf(" - BuildArtifactPath: %s", configs.BuildArtifactPath)
	log.Printf(" - KeystoreURL: %s", secureInput(configs.KeystoreURL))
	log.Printf(" - KeystorePassword: %s", secureInput(configs.KeystorePassword))
	log.Printf(" - KeystoreAlias: %s", configs.KeystoreAlias)
	log.Printf(" - PrivateKeyPassword: %s", secureInput(configs.PrivateKeyPassword))
	log.Printf(" - JarsignerOptions: %s", configs.JarsignerOptions)
	fmt.Println()
}

func (configs ConfigsModel) validate() error {
	// required
	if configs.BuildArtifactPath == "" {
		return errors.New("no BuildArtifactPath parameter specified")
	}

	buildArtifactPaths := strings.Split(configs.BuildArtifactPath, "|")
	for _, buildArtifactPath := range buildArtifactPaths {
		if exist, err := pathutil.IsPathExists(buildArtifactPath); err != nil {
			return fmt.Errorf("failed to check if BuildArtifactPath exist at: %s, error: %s", buildArtifactPath, err)
		} else if !exist {
			return fmt.Errorf("BuildArtifactPath not exist at: %s", buildArtifactPath)
		}
	}

	if configs.KeystoreURL == "" {
		return errors.New("no KeystoreURL parameter specified")
	}

	if configs.KeystorePassword == "" {
		return errors.New("no KeystorePassword parameter specified")
	}

	if configs.KeystoreAlias == "" {
		return errors.New("no KeystoreAlias parameter specified")
	}

	return nil
}

// -----------------------
// --- Functions
// -----------------------

func secureInput(str string) string {
	if str == "" {
		return ""
	}

	secureStr := func(s string, show int) string {
		runeCount := utf8.RuneCountInString(s)
		if runeCount < 6 || show == 0 {
			return strings.Repeat("*", 3)
		}
		if show*4 > runeCount {
			show = 1
		}

		sec := fmt.Sprintf("%s%s%s", s[0:show], strings.Repeat("*", 3), s[len(s)-show:len(s)])
		return sec
	}

	prefix := ""
	cont := str
	sec := secureStr(cont, 0)

	if strings.HasPrefix(str, "file://") {
		prefix = "file://"
		cont = strings.TrimPrefix(str, prefix)
		sec = secureStr(cont, 3)
	} else if strings.HasPrefix(str, "http://www.") {
		prefix = "http://www."
		cont = strings.TrimPrefix(str, prefix)
		sec = secureStr(cont, 3)
	} else if strings.HasPrefix(str, "https://www.") {
		prefix = "https://www."
		cont = strings.TrimPrefix(str, prefix)
		sec = secureStr(cont, 3)
	} else if strings.HasPrefix(str, "http://") {
		prefix = "http://"
		cont = strings.TrimPrefix(str, prefix)
		sec = secureStr(cont, 3)
	} else if strings.HasPrefix(str, "https://") {
		prefix = "https://"
		cont = strings.TrimPrefix(str, prefix)
		sec = secureStr(cont, 3)
	}

	return prefix + sec
}

func download(url, pth string) error {
	out, err := os.Create(pth)
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

func fileList(searchDir string) ([]string, error) {
	fileList := []string{}

	if err := filepath.Walk(searchDir, func(path string, f os.FileInfo, err error) error {
		fileList = append(fileList, path)
		return err
	}); err != nil {
		return []string{}, err
	}

	return fileList, nil
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
			if strings.ToLower(ext) == strings.ToLower(signExt) {
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

func zipalignBuildArtifact(zipalign, pth, dstPth string) error {
	cmdSlice := []string{zipalign, "-f", "4", pth, dstPth}

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

// -----------------------
// --- Main
// -----------------------
func main() {
	configs := createConfigsModelFromEnvs()
	configs.print()
	if err := configs.validate(); err != nil {
		failf("Issue with input: %s", err)
	}

	// Download keystore
	tmpDir, err := pathutil.NormalizedOSTempDirPath("bitrise-sign-build-artifact")
	if err != nil {
		failf("Failed to create tmp dir, error: %s", err)
	}

	keystorePath := ""
	if strings.HasPrefix(configs.KeystoreURL, "file://") {
		pth := strings.TrimPrefix(configs.KeystoreURL, "file://")
		var err error
		keystorePath, err = pathutil.AbsPath(pth)
		if err != nil {
			failf("Failed to expand path (%s), error: %s", pth, err)
		}
	} else {
		log.Infof("Download keystore")
		keystorePath = path.Join(tmpDir, "keystore.jks")
		if err := download(configs.KeystoreURL, keystorePath); err != nil {
			failf("Failed to download keystore, error: %s", err)
		}
	}
	log.Printf("using keystore at: %s", keystorePath)

	keystore, err := keystore.NewHelper(keystorePath, configs.KeystorePassword, configs.KeystoreAlias)
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
	buildArtifactPaths := strings.Split(configs.BuildArtifactPath, "|")
	signedAPKPaths := make([]string, len(buildArtifactPaths))
	signedAABPaths := make([]string, len(buildArtifactPaths))

	log.Infof("signing %d Build Artifacts", len(buildArtifactPaths))
	fmt.Println()

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
		if err := keystore.SignBuildArtifact(unsignedBuildArtifactPth, unalignedBuildArtifactPth, configs.PrivateKeyPassword); err != nil {
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
		fullPath := filepath.Join(buildArtifactDir, signedArtifactName)
		switch artifactExt {
			case "apk":
				signedAPKPaths = append(signedAPKPaths, fullPath)
			case "aab":
				signedAABPaths = append(signedAABPaths, fullPath)
			default:
				signedAPKPaths = append(signedAPKPaths, fullPath)
		}
		if err := zipalignBuildArtifact(zipalign, unalignedBuildArtifactPth, signedAPKPaths[i]); err != nil {
			failf("Failed to zipalign Build Artifact, error: %s", err)
		}
		fmt.Println()
		// ---
	}

	signedBuildArtifactPth := strings.Join(signedAPKPaths, "|")

	if err := tools.ExportEnvironmentWithEnvman("BITRISE_SIGNED_APK_PATH", signedBuildArtifactPth); err != nil {
		log.Warnf("Failed to export Build Artifact, error: %s", err)
	}
	log.Donef("The Signed Build Artifact path is now available in the Environment Variable: BITRISE_SIGNED_APK_PATH (value: %s)", signedBuildArtifactPth)

	if err := tools.ExportEnvironmentWithEnvman("BITRISE_APK_PATH", signedBuildArtifactPth); err != nil {
		log.Warnf("Failed to export Build Artifact, error: %s", err)
	}
	log.Donef("The Signed Build Artifact path is now available in the Environment Variable: BITRISE_APK_PATH (value: %s)", signedBuildArtifactPth)
}
