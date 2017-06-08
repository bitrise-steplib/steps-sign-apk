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
	ApkPath            string
	KeystoreURL        string
	KeystorePassword   string
	KeystoreAlias      string
	PrivateKeyPassword string
	JarsignerOptions   string
}

func createConfigsModelFromEnvs() ConfigsModel {
	return ConfigsModel{
		ApkPath:            os.Getenv("apk_path"),
		KeystoreURL:        os.Getenv("keystore_url"),
		KeystorePassword:   os.Getenv("keystore_password"),
		KeystoreAlias:      os.Getenv("keystore_alias"),
		PrivateKeyPassword: os.Getenv("private_key_password"),
		JarsignerOptions:   os.Getenv("jarsigner_options"),
	}
}

func (configs ConfigsModel) print() {
	fmt.Println()
	log.Infof("Configs:")
	log.Printf(" - ApkPath: %s", configs.ApkPath)
	log.Printf(" - KeystoreURL: %s", secureInput(configs.KeystoreURL))
	log.Printf(" - KeystorePassword: %s", secureInput(configs.KeystorePassword))
	log.Printf(" - KeystoreAlias: %s", configs.KeystoreAlias)
	log.Printf(" - PrivateKeyPassword: %s", secureInput(configs.PrivateKeyPassword))
	log.Printf(" - JarsignerOptions: %s", configs.JarsignerOptions)
	fmt.Println()
}

func (configs ConfigsModel) validate() error {
	// required
	if configs.ApkPath == "" {
		return errors.New("no ApkPath parameter specified")
	}

	apkPaths := strings.Split(configs.ApkPath, "|")
	for _, apkPath := range apkPaths {
		if exist, err := pathutil.IsPathExists(apkPath); err != nil {
			return fmt.Errorf("failed to check if ApkPath exist at: %s, error: %s", apkPath, err)
		} else if !exist {
			return fmt.Errorf("ApkPath not exist at: %s", apkPath)
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

func listFilesInAPK(aapt, pth string) ([]string, error) {
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

func removeFilesFromAPK(aapt, pth string, files []string) error {
	cmdSlice := append([]string{aapt, "remove", pth}, files...)

	prinatableCmd := command.PrintableCommandArgs(false, cmdSlice)
	log.Printf("=> %s", prinatableCmd)

	out, err := keystore.ExecuteForOutput(cmdSlice)
	if err != nil && errorutil.IsExitStatusError(err) {
		return errors.New(out)
	}
	return err
}

func isAPKSigned(aapt, pth string) (bool, error) {
	filesInAPK, err := listFilesInAPK(aapt, pth)
	if err != nil {
		return false, err
	}

	metaFiles := filterMETAFiles(filesInAPK)

	for _, metaFile := range metaFiles {
		ext := filepath.Ext(metaFile)
		if strings.EqualFold(ext, ".dsa") || strings.EqualFold(ext, ".rsa") {
			return true, nil
		}
	}
	return false, nil
}

func unsignAPK(aapt, pth string) error {
	filesInAPK, err := listFilesInAPK(aapt, pth)
	if err != nil {
		return err
	}

	metaFiles := filterMETAFiles(filesInAPK)
	signingFiles := filterSigningFiles(metaFiles)

	if len(signingFiles) == 0 {
		log.Printf("APK is not signed")
		return nil
	}

	return removeFilesFromAPK(aapt, pth, signingFiles)
}

func zipalignAPK(zipalign, pth, dstPth string) error {
	cmdSlice := []string{zipalign, "-f", "4", pth, dstPth}

	prinatableCmd := command.PrintableCommandArgs(false, cmdSlice)
	log.Printf("=> %s", prinatableCmd)

	_, err := keystore.ExecuteForOutput(cmdSlice)
	return err
}

func prettyAPKBasename(apkPth string) string {
	apkBasenameWithExt := path.Base(apkPth)
	apkExt := filepath.Ext(apkBasenameWithExt)
	apkBasename := strings.TrimSuffix(apkBasenameWithExt, apkExt)
	apkBasename = strings.TrimSuffix(apkBasename, "-unsigned")
	return apkBasename
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
	tmpDir, err := pathutil.NormalizedOSTempDirPath("bitrise-sign-apk")
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

	// Sign apks
	apkPaths := strings.Split(configs.ApkPath, "|")
	signedAPKPaths := make([]string, len(apkPaths))

	log.Infof("signing %d apks", len(apkPaths))
	fmt.Println()

	for i, apkPath := range apkPaths {
		log.Donef("%d/%d signing %s", i+1, len(apkPaths), apkPath)
		fmt.Println()

		apkDir := path.Dir(apkPath)
		apkBasename := prettyAPKBasename(apkPath)

		// unsign apk
		unsignedAPKPth := filepath.Join(tmpDir, "unsigned.apk")
		if err := command.CopyFile(apkPath, unsignedAPKPth); err != nil {
			failf("Failed to copy apk, error: %s", err)
		}

		isSigned, err := isAPKSigned(aapt, unsignedAPKPth)
		if err != nil {
			failf("Failed to check if apk is signed, error: %s", err)
		}

		if isSigned {
			log.Printf("Signature file (DSA or RSA) found in META-INF, unsigning the apk...")
			if err := unsignAPK(aapt, unsignedAPKPth); err != nil {
				failf("Failed to unsign APK, error: %s", err)
			}
			fmt.Println()
		} else {
			log.Printf("No signature file (DSA or RSA) found in META-INF, skipping apk unsign...")
			fmt.Println()
		}
		// ---

		// sign apk
		unalignedAPKPth := filepath.Join(tmpDir, "unaligned.apk")
		log.Infof("Sign APK")
		if err := keystore.SignAPK(unsignedAPKPth, unalignedAPKPth, configs.PrivateKeyPassword); err != nil {
			failf("Failed to sign APK, error: %s", err)
		}
		fmt.Println()

		log.Infof("Verify APK")
		if err := keystore.VerifyAPK(unalignedAPKPth); err != nil {
			failf("Failed to verify APK, error: %s", err)
		}
		fmt.Println()

		log.Infof("Zipalign APK")
		signedAPKPaths[i] = filepath.Join(apkDir, apkBasename+"-bitrise-signed.apk")
		if err := zipalignAPK(zipalign, unalignedAPKPth, signedAPKPaths[i]); err != nil {
			failf("Failed to zipalign APK, error: %s", err)
		}
		fmt.Println()
		//
	}

	signedAPKPth := strings.Join(signedAPKPaths, "|")

	if err := tools.ExportEnvironmentWithEnvman("BITRISE_SIGNED_APK_PATH", signedAPKPth); err != nil {
		log.Warnf("Failed to export APK, error: %s", err)
	}
	log.Donef("The Signed APK path is now available in the Environment Variable: BITRISE_SIGNED_APK_PATH (value: %s)", signedAPKPth)

	if err := tools.ExportEnvironmentWithEnvman("BITRISE_APK_PATH", signedAPKPth); err != nil {
		log.Warnf("Failed to export APK, error: %s", err)
	}
	log.Donef("The Signed APK path is now available in the Environment Variable: BITRISE_APK_PATH (value: %s)", signedAPKPth)
}
