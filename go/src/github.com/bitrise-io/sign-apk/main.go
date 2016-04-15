package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/bitrise-io/go-utils/cmdex"
	"github.com/bitrise-io/go-utils/pathutil"
	shellquote "github.com/kballard/go-shellquote"
)

const jarsigner = "/usr/bin/jarsigner"

// -----------------------
// --- Functions
// -----------------------

func logFail(format string, v ...interface{}) {
	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("\x1b[31;1m%s\x1b[0m\n", errorMsg)
	os.Exit(1)
}

func logWarn(format string, v ...interface{}) {
	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("\x1b[33;1m%s\x1b[0m\n", errorMsg)
}

func logInfo(format string, v ...interface{}) {
	fmt.Println()
	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("\x1b[34;1m%s\x1b[0m\n", errorMsg)
}

func logDetails(format string, v ...interface{}) {
	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("  %s\n", errorMsg)
}

func logDone(format string, v ...interface{}) {
	errorMsg := fmt.Sprintf(format, v...)
	fmt.Printf("  \x1b[32;1m%s\x1b[0m\n", errorMsg)
}

func validateRequiredInput(key string) string {
	value := os.Getenv(key)
	if value == "" {
		logFail("missing required input: %s", key)
	}
	return value
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

func isToolInstalled(tool string) (bool, error) {
	toolPath, err := cmdex.RunCommandAndReturnCombinedStdoutAndStderr("which", tool)
	if err != nil {
		return false, err
	}
	return (toolPath != ""), nil
}

func ensureZipInstalled() error {
	osName, err := osName()
	if err != nil {
		return err
	}

	tools := []string{"zip", "unzip"}
	for _, tool := range tools {
		if installed, err := isToolInstalled(tool); err != nil {
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
			logWarn("Failed to close file: %s, err: %s", out, err)
		}
	}()

	resp, err := http.Get(url)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logWarn("Failed to close response body, err: %s", err)
		}
	}()
	if err != nil {
		return err
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

func jarsignerSign(keystore, storePass, keyPass, optionsStr, signedApk, apk, keystoreAlias string) error {
	args := []string{
		"-keystore", keystore,
		"-storepass", storePass,
		"-keypass", keyPass,
	}
	if optionsStr != "" {
		options, err := shellquote.Split(optionsStr)
		if err != nil {
			return err
		}

		args = append(args, options...)
	}
	args = append(args, "-signedjar", signedApk, apk, keystoreAlias)

	return cmdex.RunCommand(jarsigner, args...)
}

func exportLatestZipalign() (string, error) {
	thisScriptDir := os.Getenv("THIS_SCRIPT_DIR")

	exportScriptPth := path.Join(thisScriptDir, "export_latest_zipalign.rb")
	return cmdex.RunCommandAndReturnCombinedStdoutAndStderr("ruby", exportScriptPth)
}

func zipalign(tmpSignedApk, signedApk string) error {
	zipalign, err := exportLatestZipalign()
	if err != nil {
		fmt.Printf("zipalign err: %s\n", zipalign)
		return err
	}

	return cmdex.RunCommand(zipalign, "-f", "4", tmpSignedApk, signedApk)
}

func jarsignerVerify(signedAPK string) (bool, error) {
	out, err := cmdex.RunCommandAndReturnCombinedStdoutAndStderr(jarsigner, "-verify", "-verbose", "-certs", signedAPK)
	if err != nil {
		return false, err
	}

	return strings.Contains(out, "jar verified"), nil
}

func zip(targetDir, targetRelPathToZip, zipPath string) error {
	zipCmd := exec.Command("zip", "-rTy", zipPath, targetRelPathToZip)
	zipCmd.Dir = targetDir
	if out, err := zipCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Zip failed, out: %s, err: %#v", out, err)
	}
	return nil
}

func unZip(zipDir, zipName, unzipPath string) error {
	zipCmd := exec.Command("unzip", zipName, "-d", unzipPath)
	zipCmd.Dir = zipDir
	if out, err := zipCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Zip failed, out: %s, err: %#v", out, err)
	}
	return nil
}

// -----------------------
// --- Main
// -----------------------
func main() {
	//
	// Validate options
	logInfo("Configs:")
	logDetails("apk_path: %s", os.Getenv("apk_path"))
	logDetails("keystore_url: %s", "***")
	logDetails("keystore_password: %s", "***")
	logDetails("keystore_alias: %s", "***")
	logDetails("private_key_password: %s", "***")
	logDetails("jarsigner_options: %s", os.Getenv("jarsigner_options"))

	apkPath := validateRequiredInput("apk_path")
	keystoreURL := validateRequiredInput("keystore_url")
	keystorePassword := validateRequiredInput("keystore_password")
	keystoreAlias := validateRequiredInput("keystore_alias")
	privateKeyPassword := validateRequiredInput("private_key_password")
	jarsignerOptions := os.Getenv("jarsigner_options")

	tmpDir, err := pathutil.NormalizedOSTempDirPath("bitrise-sign-apk")
	if err != nil {
		logFail("Failed to create tmp dir, err: %s", err)
	}

	apkBaseNameWithExt := path.Base(apkPath)
	apkBaseName := strings.TrimSuffix(apkBaseNameWithExt, path.Ext(apkBaseNameWithExt))
	apkDirName := path.Dir(apkPath)

	// Ensure installed zip and unzip
	logInfo("Ensure installed zip and unzip")
	if err := ensureZipInstalled(); err != nil {
		logFail("Failed to ensure installed zip and upnzi, error: %s", err)
	}
	logDetails("zip and unzip tools are installed")

	// Download keystore
	logInfo("Download keystore")
	keystorePath := ""
	if strings.HasPrefix(keystoreURL, "file://") {
		pth := strings.TrimPrefix(keystoreURL, "file://")
		keystorePath, err = pathutil.AbsPath(pth)
		if err != nil {
			logFail("Failed to expand path (%s), err: %s", pth, err)
		}
	} else {
		keystorePath = path.Join(tmpDir, "keystore.jks")
		if err := download(keystoreURL, keystorePath); err != nil {
			logFail("Failed to download keystore, err: %s", err)
		}
	}
	logDetails("using keystore: %s", keystorePath)

	// Remove previous sign
	logInfo("Remove previous sign")
	tmpApkZIPName := fmt.Sprintf("bitrise-tmp-%s.zip", apkBaseName)
	tmpApkZIPPath := path.Join(tmpDir, tmpApkZIPName)
	if err := cmdex.CopyFile(apkPath, tmpApkZIPPath); err != nil {
		logFail("Failed to copy APK to (%s), err: %s", tmpApkZIPPath, err)
	}
	logDetails("tmp APK zip created at: (%s)", tmpApkZIPPath)

	tmpApkUnzipDirName := fmt.Sprintf("bitrise-tmp-%s", apkBaseName)
	tmpApkUnzipPath := path.Join(tmpDir, tmpApkUnzipDirName)
	if err := unZip(tmpDir, tmpApkZIPName, tmpApkUnzipDirName); err != nil {
		logFail("Failed to unzip (%s), err: %s", tmpApkZIPPath, err)
	}
	logDetails("tmp APK zip unzipped at: (%s)", tmpApkUnzipPath)

	tmpApkMetaDir := path.Join(tmpApkUnzipPath, "META-INF")
	if exist, err := pathutil.IsDirExists(tmpApkMetaDir); err != nil {
		logFail("Failed to check if path (%s) exist, err: %s", tmpApkMetaDir, err)
	} else if exist {
		logWarn("APK already signed removing it...")
		if err := cmdex.RemoveDir(tmpApkMetaDir); err != nil {
			logFail("Failed to remove META-INF dir (%s), err: %s", tmpApkMetaDir, err)
		}

		unsignedApkName := fmt.Sprintf("bitrise-unsigned-%s", apkBaseNameWithExt)
		unsignedApkPath := path.Join(tmpDir, unsignedApkName)
		if err := zip(tmpDir, tmpApkUnzipDirName, unsignedApkName); err != nil {
			logFail("Failed to zip (%s), err: %s", unsignedApkPath, err)
		}

		logDetails("Using unsigned APK (%s)...", unsignedApkPath)
		apkPath = unsignedApkPath
	}

	// Sign apk
	logInfo("Sign APK")
	signedApkName := fmt.Sprintf("bitrise-signed-%s", apkBaseNameWithExt)
	tmpSignedApkPath := path.Join(tmpDir, signedApkName)
	signedApkPath := path.Join(apkDirName, signedApkName)
	if err := jarsignerSign(keystorePath, keystorePassword, privateKeyPassword, jarsignerOptions, tmpSignedApkPath, apkPath, keystoreAlias); err != nil {
		logFail("Failed to sign APK, err: %s", err)
	}
	logDetails("signed APK: %s", tmpSignedApkPath)

	// Now zipalign it
	logInfo("Aligning the APK")
	if err := zipalign(tmpSignedApkPath, signedApkPath); err != nil {
		logFail("Failed to zipaling APK, err: %s", err)
	}
	logDetails("aligned signed APK: %s", signedApkPath)

	// Verifying APK
	logInfo("Verifying the signed APK")
	verified, err := jarsignerVerify(signedApkPath)
	if err != nil {
		logFail("Failed to verify APK, err: %s", err)
	}
	if !verified {
		logFail("APK not verified, signing failed")
	}

	// Exporting signed ipa
	logDone("Signed APK created at: %s", signedApkPath)
	if err := exportEnvironmentWithEnvman("BITRISE_SIGNED_APK_PATH", signedApkPath); err != nil {
		logWarn("Failed to export APK, err: %s", err)
	}
}
