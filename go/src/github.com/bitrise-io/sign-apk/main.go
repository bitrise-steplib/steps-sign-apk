package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/cmdex"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/sign-apk/keystore"
	log "github.com/bitrise-io/sign-apk/logger"
	"github.com/bitrise-io/sign-apk/run"
	version "github.com/hashicorp/go-version"
)

const jarsigner = "/usr/bin/jarsigner"

var signingFileExts = []string{".mf", ".rsa", ".dsa", ".ec", ".sf"}

// -----------------------
// --- Functions
// -----------------------

func validateRequiredInput(key, value string) {
	if value == "" {
		log.Fail("Missing required input: %s", key)
	}
}

func exportEnvironmentWithEnvman(keyStr, valueStr string) error {
	envman := exec.Command("envman", "add", "--key", keyStr)
	envman.Stdin = strings.NewReader(valueStr)
	envman.Stdout = os.Stdout
	envman.Stderr = os.Stderr
	return envman.Run()
}

func osName() (string, error) {
	return cmdex.RunCommandAndReturnCombinedStdoutAndStderr("uname", "-s")
}

func aptGetInstall(tool string) error {
	return cmdex.RunCommand("apt-get", "install", tool)
}

func isToolInstalled(tool string) bool {
	toolPath, err := cmdex.RunCommandAndReturnCombinedStdoutAndStderr("which", tool)
	if err != nil {
		return false
	}
	return (toolPath != "")
}

func ensureZipInstalled() error {
	osName, err := osName()
	if err != nil {
		return err
	}

	tools := []string{"zip", "unzip"}
	for _, tool := range tools {
		if installed := isToolInstalled(tool); err != nil {
			return err
		} else if !installed {
			if osName == "Darwin" {
				return fmt.Errorf("tool (%s) should be installed on %s", tool, osName)
			} else if osName == "Linux" {
				if err := aptGetInstall(tool); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("unkown os name: %s", osName)
			}
		}
	}

	return nil
}

func download(url, pth string) error {
	out, err := os.Create(pth)
	defer func() {
		if err := out.Close(); err != nil {
			log.Warn("Failed to close file: %s, error: %s", out, err)
		}
	}()

	resp, err := http.Get(url)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warn("Failed to close response body, error: %s", err)
		}
	}()
	if err != nil {
		return err
	}

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

func latestBuildToolsDir(androidHome string) (string, error) {
	buildTools := filepath.Join(androidHome, "build-tools")
	pattern := filepath.Join(buildTools, "*")

	buildToolsDirs, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	var latestVersion version.Version
	for _, buildToolsDir := range buildToolsDirs {
		versionStr := strings.TrimPrefix(buildToolsDir, buildTools+"/")

		version, err := version.NewVersion(versionStr)
		if err != nil {
			return "", err
		}

		if latestVersion.String() == "" || version.GreaterThan(&latestVersion) {
			latestVersion = *version
		}
	}

	if latestVersion.String() == "" {
		return "", errors.New("failed to find latest build-tools dir")
	}

	return filepath.Join(buildTools, latestVersion.String()), nil
}

func listFilesInAPK(aapt, pth string) ([]string, error) {
	cmdSlice := []string{aapt, "list", pth}
	out, err := run.ExecuteForOutput(cmdSlice)
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

func removeFilesFromAPK(aapt, pth string, files []string) error {
	cmdSlice := append([]string{aapt, "remove", pth}, files...)

	prinatableCmd := cmdex.PrintableCommandArgs(false, cmdSlice)
	log.Details("=> %s", prinatableCmd)

	out, err := run.ExecuteForOutput(cmdSlice)
	if err != nil && errorutil.IsExitStatusError(err) {
		return errors.New(out)
	}
	return err
}

func unsignAPK(aapt, pth string) error {
	filesInAPK, err := listFilesInAPK(aapt, pth)
	if err != nil {
		return err
	}

	metaFiles := filterMETAFiles(filesInAPK)
	signingFiles := filterSigningFiles(metaFiles)

	if len(signingFiles) == 0 {
		log.Details("APK is not signed")
		return nil
	}

	return removeFilesFromAPK(aapt, pth, signingFiles)
}

func zipalignAPK(zipalign, pth, dstPth string) error {
	cmdSlice := []string{zipalign, "-f", "4", pth, dstPth}

	prinatableCmd := cmdex.PrintableCommandArgs(false, cmdSlice)
	log.Details("=> %s", prinatableCmd)

	_, err := run.ExecuteForOutput(cmdSlice)
	return err
}

// -----------------------
// --- Main
// -----------------------
func main() {

	//
	// Validate options
	apkPath := os.Getenv("apk_path")
	keystoreURL := os.Getenv("keystore_url")
	keystorePassword := os.Getenv("keystore_password")
	keystoreAlias := os.Getenv("keystore_alias")
	privateKeyPassword := os.Getenv("private_key_password")
	jarsignerOptions := os.Getenv("jarsigner_options")

	log.Configs(apkPath, keystoreURL, keystorePassword, keystoreAlias, privateKeyPassword, jarsignerOptions)

	if jarsignerOptions != "" {
		fmt.Println("")
		log.Warn("jarsigner_options is deprecated, options are detected from the keystore")
	}

	validateRequiredInput("apk_path", apkPath)
	validateRequiredInput("keystore_url", keystoreURL)
	validateRequiredInput("keystore_password", keystorePassword)
	validateRequiredInput("keystore_alias", keystoreAlias)
	validateRequiredInput("private_key_password", privateKeyPassword)
	fmt.Println("")

	//
	// Prepare paths
	tmpDir, err := pathutil.NormalizedOSTempDirPath("bitrise-sign-apk")
	if err != nil {
		log.Fail("Failed to create tmp dir, error: %s", err)
	}
	apkDir := path.Dir(apkPath)
	apkBasenameWithExt := path.Base(apkPath)
	apkExt := filepath.Ext(apkBasenameWithExt)
	apkBasename := strings.TrimSuffix(apkBasenameWithExt, apkExt)

	//
	// Download keystore
	keystorePath := ""
	if strings.HasPrefix(keystoreURL, "file://") {
		pth := strings.TrimPrefix(keystoreURL, "file://")
		var err error
		keystorePath, err = pathutil.AbsPath(pth)
		if err != nil {
			log.Fail("Failed to expand path (%s), error: %s", pth, err)
		}
	} else {
		log.Info("Download keystore")
		keystorePath = path.Join(tmpDir, "keystore.jks")
		if err := download(keystoreURL, keystorePath); err != nil {
			log.Fail("Failed to download keystore, error: %s", err)
		}
	}
	log.Details("using keystore at: %s", keystorePath)
	fmt.Println("")

	keystore, err := keystore.NewKeystoreModel(keystorePath, keystorePassword, keystoreAlias)
	if err != nil {
		log.Fail("Failed to create keystore, error: %s", err)
	}

	//
	// Find Android tools
	androidHome := os.Getenv("ANDROID_HOME")
	log.Details("android_home: %s", androidHome)

	latestBuildToolsDir, err := latestBuildToolsDir(androidHome)
	if err != nil {
		log.Fail("failed to find latest build-tools")
	}
	log.Details("build_tools: %s", latestBuildToolsDir)

	aapt := filepath.Join(latestBuildToolsDir, "aapt")
	if exist, err := pathutil.IsPathExists(aapt); err != nil {
		log.Fail("Failed to find aapt path, error: %s", err)
	} else if !exist {
		log.Fail("aapt not found at: %s", aapt)
	}
	log.Details("aapt: %s", aapt)

	zipalign := filepath.Join(latestBuildToolsDir, "zipalign")
	if exist, err := pathutil.IsPathExists(zipalign); err != nil {
		log.Fail("Failed to find zipalign path, error: %s", err)
	} else if !exist {
		log.Fail("zipalign not found at: %s", zipalign)
	}
	log.Details("zipalign: %s", zipalign)

	//
	// Resign apk
	unsignedAPKPth := filepath.Join(tmpDir, "unsigned.apk")
	cmdex.CopyFile(apkPath, unsignedAPKPth)

	log.Info("Unsign APK if signed")
	if err := unsignAPK(aapt, unsignedAPKPth); err != nil {
		log.Fail("Failed to unsign APK, error: %s", err)
	}
	log.Done("unsiged")

	unalignedAPKPth := filepath.Join(tmpDir, "unaligned.apk")
	log.Info("Sign APK")
	if err := keystore.SignAPK(unsignedAPKPth, unalignedAPKPth, privateKeyPassword); err != nil {
		log.Fail("Failed to sign APK, error: %s", err)
	}
	log.Done("signed")

	log.Info("Verify APK")
	if err := keystore.VerifyAPK(unalignedAPKPth); err != nil {
		log.Fail("Failed to verify APK, error: %s", err)
	}
	log.Done("verified")

	log.Info("Zipalign APK")
	signedAPKPth := filepath.Join(apkDir, apkBasename+"bitrise-signed"+apkExt)
	if err := zipalignAPK(zipalign, unalignedAPKPth, signedAPKPth); err != nil {
		log.Fail("Failed to zipalign APK, error: %s", err)
	}
	log.Done("zipaligned")

	// Exporting signed ipa
	fmt.Println("")
	log.Done("Signed APK created at: %s", signedAPKPth)
	if err := exportEnvironmentWithEnvman("BITRISE_SIGNED_APK_PATH", signedAPKPth); err != nil {
		log.Warn("Failed to export APK, error: %s", err)
	}
}
