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

	"github.com/bitrise-io/go-android/sdk"
	"github.com/bitrise-io/go-steputils/v2/export"
	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
	"github.com/bitrise-io/go-utils/v2/fileutil"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
	"github.com/bitrise-steplib/steps-sign-apk/keystore"
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

	VerboseLog          bool   `env:"verbose_log,opt[true,false]"`
	PageAlign           string `env:"page_align,opt[automatic,true,false]"`
	SignerScheme        string `env:"signer_scheme,opt[automatic,v2,v3,v4]"`
	DebuggablePermitted string `env:"debuggable_permitted,opt[true,false]"`
	SignerTool          string `env:"signer_tool,opt[automatic,apksigner,jarsigner]"`

	// Deprecated
	APKPath string `env:"apk_path"`
}

type codeSignerTool string

const (
	apksignerSignerTool codeSignerTool = "apksigner"
	jarsignerSignerTool codeSignerTool = "jarsigner"
	automaticSignerTool codeSignerTool = "automatic"
)

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

func download(logger log.Logger, url, pth string) error {
	out, err := os.Create(pth)
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			logger.Warnf("Failed to close file: %s, error: %s", out, err)
		}
	}()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("Failed to close response body, error: %s", err)
		}
	}()

	_, err = io.Copy(out, resp.Body)

	return err
}

func listFilesInBuildArtifact(runner keystore.Runner, aapt, pth string) ([]string, error) {
	cmdSlice := []string{aapt, "list", pth}
	out, err := runner.ExecuteForOutput(cmdSlice)
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
	var signingFiles []string
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

func removeFilesFromBuildArtifact(runner keystore.Runner, logger log.Logger, aapt, pth string, files []string) error {
	cmdSlice := append([]string{aapt, "remove", pth}, files...)

	logger.Printf("=> %s", runner.PrintableCommandArgs(cmdSlice))

	out, err := runner.ExecuteForOutput(cmdSlice)
	if err != nil {
		var exitErr *command.ExitStatusError
		if errors.As(err, &exitErr) {
			return errors.New(out)
		}

		return err
	}

	return nil
}

func isBuildArtifactSigned(runner keystore.Runner, aapt, pth string) (bool, error) {
	filesInBuildArtifact, err := listFilesInBuildArtifact(runner, aapt, pth)
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

func unsignBuildArtifact(runner keystore.Runner, logger log.Logger, aapt, pth string) error {
	filesInBuildArtifact, err := listFilesInBuildArtifact(runner, aapt, pth)
	if err != nil {
		return err
	}

	metaFiles := filterMETAFiles(filesInBuildArtifact)
	signingFiles := filterSigningFiles(metaFiles)

	if len(signingFiles) == 0 {
		logger.Printf("Build Artifact is not signed")

		return nil
	}

	return removeFilesFromBuildArtifact(runner, logger, aapt, pth, signingFiles)
}

func prettyBuildArtifactBasename(buildArtifactPth string) string {
	buildArtifactBasenameWithExt := path.Base(buildArtifactPth)
	buildArtifactExt := filepath.Ext(buildArtifactBasenameWithExt)
	buildArtifactBasename := strings.TrimSuffix(buildArtifactBasenameWithExt, buildArtifactExt)
	buildArtifactBasename = strings.TrimSuffix(buildArtifactBasename, "-unsigned")

	return buildArtifactBasename
}

func failf(logger log.Logger, format string, v ...interface{}) {
	logger.Errorf(format, v...)
	os.Exit(1)
}

func handleDeprecatedInputs(logger log.Logger, cfg *configs) {
	if cfg.APKPath != "" {
		logger.Warnf("step input 'APK file path' (apk_path) is deprecated and will be removed on 20 August 2019, use 'APK or App Bundle file path' (android_app) instead!")
		cfg.BuildArtifactPath = cfg.APKPath
	}
}

func validate(logger log.Logger, pathChecker pathutil.PathChecker, cfg configs) error {
	buildArtifactPaths := parseAppList(cfg.BuildArtifactPath)
	for _, buildArtifactPath := range buildArtifactPaths {
		if exist, err := pathChecker.IsPathExists(buildArtifactPath); err != nil {
			return fmt.Errorf("failed to check if BuildArtifactPath exist at: %s, error: %s", buildArtifactPath, err)
		} else if !exist {
			return fmt.Errorf("BuildArtifactPath not exist at: %s", buildArtifactPath)
		}

		artifactExt := path.Ext(buildArtifactPath)
		signAAB := strings.EqualFold(artifactExt, ".aab")
		if cfg.SignerTool == "apksigner" && signAAB {
			failf(logger, "signer tool apksigner does not support signing AABs, please use automatic or jarsigner instead")
		}
	}

	return nil
}

// -----------------------
// --- Main
// -----------------------
func main() {
	logger := log.NewLogger()
	envRepo := env.NewRepository()
	cmdFactory := command.NewFactory(envRepo)
	pathChecker := pathutil.NewPathChecker()
	pathModifier := pathutil.NewPathModifier()
	pathProvider := pathutil.NewPathProvider()
	fileManager := fileutil.NewFileManager()
	exporter := export.NewExporter(cmdFactory, fileManager)
	runner := keystore.NewRunner(logger, cmdFactory)

	var cfg configs
	if err := stepconf.NewInputParser(envRepo).Parse(&cfg); err != nil {
		failf(logger, "Process config: failed to parse input: %s", err)
	}
	pageAlignConfig := parsePageAlign(cfg.PageAlign)

	stepconf.Print(cfg)
	logger.EnableDebugLog(cfg.VerboseLog)
	handleDeprecatedInputs(logger, &cfg)
	fmt.Println()

	if err := validate(logger, pathChecker, cfg); err != nil {
		failf(logger, "Process config: failed to validate input: %s", err)
	}

	// Download keystore
	tmpDir, err := pathProvider.CreateTempDir("bitrise-sign-build-artifact")
	if err != nil {
		failf(logger, "Run: failed to create tmp dir: %s", err)
	}

	keystorePath := ""
	if strings.HasPrefix(cfg.KeystoreURL, "file://") {
		pth := strings.TrimPrefix(cfg.KeystoreURL, "file://")
		var err error
		keystorePath, err = pathModifier.AbsPath(pth)
		if err != nil {
			failf(logger, "Run: failed to expand path (%s): %s", pth, err)
		}
	} else {
		logger.Infof("Download keystore")
		keystorePath = path.Join(tmpDir, "keystore.jks")
		if err := download(logger, cfg.KeystoreURL, keystorePath); err != nil {
			failf(logger, "Run: failed to download keystore: %s", err)
		}
	}
	logger.Printf("using keystore at: %s", keystorePath)

	ks, err := keystore.NewHelper(runner, pathChecker, keystorePath, cfg.KeystorePassword, cfg.KeystoreAlias)
	if err != nil {
		failf(logger, "Run: failed to create keystore helper: %s", err)
	}
	// ---

	// Find Android tools
	androidHome := os.Getenv("ANDROID_HOME")
	logger.Printf("android_home: %s", androidHome)

	androidSDK, err := sdk.New(androidHome)
	if err != nil {
		failf(logger, "Run: failed to create SDK model: %s", err)
	}

	aapt, err := androidSDK.LatestBuildToolPath("aapt")
	if err != nil {
		failf(logger, "Run: failed to find AAPT path: %s", err)
	}
	logger.Printf("aapt: %s", aapt)

	zipalign, err := androidSDK.LatestBuildToolPath("zipalign")
	if err != nil {
		failf(logger, "Run: failed to find zipalign path: %s", err)
	}
	logger.Printf("zipalign: %s", zipalign)

	apkSigner, err := NewKeystoreSignatureConfiguration(logger, cmdFactory, keystorePath, cfg.KeystorePassword, cfg.KeystoreAlias, cfg.PrivateKeyPassword, cfg.DebuggablePermitted, cfg.SignerScheme)
	if err != nil {
		failf(logger, "Run: failed to create signature configuration: %s", err)
	}
	// ---

	// Sign build artifacts
	buildArtifactPaths := parseAppList(cfg.BuildArtifactPath)
	signedAPKPaths := make([]string, 0)
	signedAABPaths := make([]string, 0)

	fmt.Println()
	logger.Infof("Signing %d Build Artifacts", len(buildArtifactPaths))

	if len(buildArtifactPaths) > 1 && cfg.OutputName != "" {
		logger.Warnf("output_name is set and more than one artifact found, disabling artifact renaming as it would result in overwriting exported artifacts")
		fmt.Println()
		cfg.OutputName = ""
	}

	for i, buildArtifactPath := range buildArtifactPaths {
		artifactExt := path.Ext(buildArtifactPath)
		logger.Donef("%d/%d signing %s", i+1, len(buildArtifactPaths), buildArtifactPath)
		fmt.Println()

		buildArtifactDir := path.Dir(buildArtifactPath)
		buildArtifactBasename := prettyBuildArtifactBasename(buildArtifactPath)

		// unsign build artifact
		unsignedBuildArtifactPth := filepath.Join(tmpDir, "unsigned"+artifactExt)
		if err := fileManager.CopyFile(buildArtifactPath, unsignedBuildArtifactPth, &fileutil.CopyOptions{Overwrite: true}); err != nil {
			failf(logger, "Run: failed to copy build artifact: %s", err)
		}

		signAAB := strings.EqualFold(artifactExt, ".aab")
		signerTool := cfg.SignerTool
		if signerTool == string(automaticSignerTool) {
			if signAAB {
				signerTool = string(jarsignerSignerTool)
			} else {
				signerTool = string(apksignerSignerTool)
			}
		}

		if signerTool == string(jarsignerSignerTool) {
			isSigned, err := isBuildArtifactSigned(runner, aapt, unsignedBuildArtifactPth)
			if err != nil {
				failf(logger, "Run: failed to check if build artifact is signed: %s", err)
			}

			if isSigned {
				logger.Printf("Signature file (DSA or RSA) found in META-INF, unsigning the build artifact...")
				if err := unsignBuildArtifact(runner, logger, aapt, unsignedBuildArtifactPth); err != nil {
					failf(logger, "Run: failed to un-sign Build Artifact: %s", err)
				}
				fmt.Println()
			} else {
				logger.Printf("No signature file (DSA or RSA) found in META-INF, skipping build artifact unsign...")
				fmt.Println()
			}
		} else {
			logger.Printf("Skipping removal of existing signature as apksigner can re-sign already signed apk.")
		}

		var fullPath string
		if signerTool == string(apksignerSignerTool) {
			fullPath = signAPK(logger, runner, fileManager, zipalign, unsignedBuildArtifactPth, buildArtifactDir, buildArtifactBasename, artifactExt, cfg.OutputName, apkSigner, pageAlignConfig)
		} else {
			fullPath = signJarSigner(logger, runner, fileManager, zipalign, tmpDir, unsignedBuildArtifactPth, buildArtifactDir, buildArtifactBasename, artifactExt, cfg.PrivateKeyPassword, cfg.OutputName, ks, pageAlignConfig)
		}

		if signAAB {
			signedAABPaths = append(signedAABPaths, fullPath)
		} else {
			signedAPKPaths = append(signedAPKPaths, fullPath)
		}

		fmt.Println()
		// ---
	}

	joinedAPKOutputPaths := strings.Join(signedAPKPaths, "|")
	joinedAABOutputPaths := strings.Join(signedAABPaths, "|")

	// APK
	if len(signedAPKPaths) > 0 {
		exportAPK(logger, &exporter, signedAPKPaths, joinedAPKOutputPaths)
	} else {
		logger.Debugf("No Signed APK was exported - skip BITRISE_SIGNED_APK_PATH Environment Variable export")
		logger.Debugf("No Signed APK was exported - skip BITRISE_SIGNED_APK_PATH_LIST Environment Variable export")
	}

	// AAB
	if len(signedAABPaths) > 0 {
		exportAAB(logger, &exporter, signedAABPaths, joinedAABOutputPaths)
	} else {
		logger.Debugf("No Signed AAB was exported - skip BITRISE_SIGNED_AAB_PATH Environment Variable export")
		logger.Debugf("No Signed AAB was exported - skip BITRISE_SIGNED_AAB_PATH_LIST Environment Variable export")
	}
}

func signJarSigner(logger log.Logger, runner keystore.Runner, fileManager fileutil.FileManager, zipalign, tmpDir string, unsignedBuildArtifactPth string, buildArtifactDir string, buildArtifactBasename string, artifactExt string, privateKeyPassword string, outputName string, ks keystore.Helper, pageAlignConfig pageAlignStatus) string {
	// sign build artifact
	unalignedBuildArtifactPth := filepath.Join(tmpDir, "unaligned"+artifactExt)
	logger.Infof("Sign Build Artifact with Jarsigner: %s", unsignedBuildArtifactPth)
	if err := ks.SignBuildArtifact(unsignedBuildArtifactPth, unalignedBuildArtifactPth, privateKeyPassword); err != nil {
		failf(logger, "Run: failed to sign Build Artifact: %s", err)
	}
	fmt.Println()

	logger.Infof("Verify Build Artifact")
	if err := ks.VerifyBuildArtifact(unalignedBuildArtifactPth); err != nil {
		failf(logger, "Run: failed to verify Build Artifact: %s", err)
	}
	fmt.Println()

	fullPath, err := zipAlignArtifact(runner, logger, fileManager, zipalign, unalignedBuildArtifactPth, buildArtifactDir, buildArtifactBasename, artifactExt, "signed", outputName, pageAlignConfig)
	if err != nil {
		failf(logger, "Run: failed to zipalign Build Artifact: %s", err)
	}

	return fullPath
}

func signAPK(logger log.Logger, runner keystore.Runner, fileManager fileutil.FileManager, zipalign, unsignedBuildArtifactPth, buildArtifactDir, buildArtifactBasename, artifactExt, outputName string, apkSigner SignatureConfiguration, pageAlignConfig pageAlignStatus) string {
	alignedPath, err := zipAlignArtifact(runner, logger, fileManager, zipalign, unsignedBuildArtifactPth, buildArtifactDir, buildArtifactBasename, artifactExt, "aligned", "", pageAlignConfig)
	if err != nil {
		failf(logger, "Run: failed to zipalign Build Artifact: %s", err)
	}

	signedArtifactName := fmt.Sprintf("%s-bitrise-signed%s", buildArtifactBasename, artifactExt)
	if artifactName := fmt.Sprintf("%s%s", outputName, artifactExt); outputName != "" {
		logger.Printf("- Exporting (%s) as: %s", signedArtifactName, artifactName)
		signedArtifactName = artifactName
	}
	fullPath := filepath.Join(buildArtifactDir, signedArtifactName)

	fmt.Println()
	logger.Infof("Sign Build Artifact with APKSigner: %s", alignedPath)
	err = apkSigner.SignBuildArtifact(alignedPath, fullPath)
	if err != nil {
		failf(logger, "Run: failed to build artifact: %s", err)
	}

	fmt.Println()
	logger.Infof("Verify Build Artifact")
	err = apkSigner.VerifyBuildArtifact(fullPath)
	if err != nil {
		failf(logger, "Run: failed to build artifact: %s", err)
	}

	return fullPath
}

func exportAPK(logger log.Logger, exporter *export.Exporter, signedAPKPaths []string, joinedAPKOutputPaths string) {
	last := signedAPKPaths[len(signedAPKPaths)-1]

	if err := exporter.ExportOutput("BITRISE_SIGNED_APK_PATH", last); err != nil {
		logger.Warnf("Failed to export APK (%s) error: %s", last, err)
	} else {
		logger.Donef("The Signed APK path is now available in the Environment Variable: BITRISE_SIGNED_APK_PATH (value: %s)", last)
	}

	if err := exporter.ExportOutput("BITRISE_SIGNED_APK_PATH_LIST", joinedAPKOutputPaths); err != nil {
		logger.Warnf("Failed to export APK list (%s), error: %s", joinedAPKOutputPaths, err)
	} else {
		logger.Donef("The Signed APK path list is now available in the Environment Variable: BITRISE_SIGNED_APK_PATH_LIST (value: %s)", joinedAPKOutputPaths)
	}

	if err := exporter.ExportOutput("BITRISE_APK_PATH", joinedAPKOutputPaths); err != nil {
		logger.Warnf("Failed to export APK list (%s), error: %s", joinedAPKOutputPaths, err)
	} else {
		logger.Donef("The Signed APK path is now available in the Environment Variable: BITRISE_APK_PATH (value: %s)", joinedAPKOutputPaths)
	}
}

func exportAAB(logger log.Logger, exporter *export.Exporter, signedAABPaths []string, joinedAABOutputPaths string) {
	last := signedAABPaths[len(signedAABPaths)-1]

	if err := exporter.ExportOutput("BITRISE_SIGNED_AAB_PATH", last); err != nil {
		logger.Warnf("Failed to export AAB (%s), error: %s", last, err)
	} else {
		logger.Donef("The Signed AAB path is now available in the Environment Variable: BITRISE_SIGNED_AAB_PATH (value: %s)", last)
	}

	if err := exporter.ExportOutput("BITRISE_SIGNED_AAB_PATH_LIST", joinedAABOutputPaths); err != nil {
		logger.Warnf("Failed to export AAB list (%s), error: %s", joinedAABOutputPaths, err)
	} else {
		logger.Donef("The Signed AAB path list is now available in the Environment Variable: BITRISE_SIGNED_AAB_PATH_LIST (value: %s)", joinedAABOutputPaths)
	}

	if err := exporter.ExportOutput("BITRISE_AAB_PATH", joinedAABOutputPaths); err != nil {
		logger.Warnf("Failed to export AAB list (%s), error: %s", joinedAABOutputPaths, err)
	} else {
		logger.Donef("The Signed AAB path is now available in the Environment Variable: BITRISE_AAB_PATH (value: %s)", joinedAABOutputPaths)
	}
}
