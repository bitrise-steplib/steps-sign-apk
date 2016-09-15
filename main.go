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

	"github.com/bitrise-io/go-utils/cmdex"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-steplib/steps-sign-apk/keystore"
	version "github.com/hashicorp/go-version"
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
	log.Info("Configs:")
	log.Detail(" - ApkPath: %s", configs.ApkPath)
	log.Detail(" - KeystoreURL: %s", secureInput(configs.KeystoreURL))
	log.Detail(" - KeystorePassword: %s", secureInput(configs.KeystorePassword))
	log.Detail(" - KeystoreAlias: %s", configs.KeystoreAlias)
	log.Detail(" - PrivateKeyPassword: %s", secureInput(configs.PrivateKeyPassword))
	log.Detail(" - JarsignerOptions: %s", configs.JarsignerOptions)
	fmt.Println()
}

func (configs ConfigsModel) validate() error {
	// required
	if configs.ApkPath == "" {
		return errors.New("No ApkPath parameter specified!")
	}
	if exist, err := pathutil.IsPathExists(configs.ApkPath); err != nil {
		return fmt.Errorf("Failed to check if ApkPath exist at: %s, error: %s", configs.ApkPath, err)
	} else if !exist {
		return fmt.Errorf("ApkPath not exist at: %s", configs.ApkPath)
	}

	if configs.KeystoreURL == "" {
		return errors.New("No KeystoreURL parameter specified!")
	}

	if configs.KeystorePassword == "" {
		return errors.New("No KeystorePassword parameter specified!")
	}

	if configs.KeystoreAlias == "" {
		return errors.New("No KeystoreAlias parameter specified!")
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

func exportEnvironmentWithEnvman(keyStr, valueStr string) error {
	cmd := cmdex.NewCommand("envman", "add", "--key", keyStr)
	cmd.SetStdin(strings.NewReader(valueStr))
	return cmd.Run()
}

func download(url, pth string) error {
	out, err := os.Create(pth)
	defer func() {
		if err := out.Close(); err != nil {
			log.Warn("Failed to close file: %s, error: %s", out, err)
		}
	}()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warn("Failed to close response body, error: %s", err)
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

	prinatableCmd := cmdex.PrintableCommandArgs(false, cmdSlice)
	log.Detail("=> %s", prinatableCmd)

	out, err := keystore.ExecuteForOutput(cmdSlice)
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
		log.Detail("APK is not signed")
		return nil
	}

	return removeFilesFromAPK(aapt, pth, signingFiles)
}

func zipalignAPK(zipalign, pth, dstPth string) error {
	cmdSlice := []string{zipalign, "-f", "4", pth, dstPth}

	prinatableCmd := cmdex.PrintableCommandArgs(false, cmdSlice)
	log.Detail("=> %s", prinatableCmd)

	_, err := keystore.ExecuteForOutput(cmdSlice)
	return err
}

// -----------------------
// --- Main
// -----------------------
func main() {
	configs := createConfigsModelFromEnvs()
	configs.print()
	if err := configs.validate(); err != nil {
		log.Error("Issue with input: %s", err)
		os.Exit(1)
	}

	//
	// Prepare paths
	tmpDir, err := pathutil.NormalizedOSTempDirPath("bitrise-sign-apk")
	if err != nil {
		log.Error("Failed to create tmp dir, error: %s", err)
		os.Exit(1)
	}
	apkDir := path.Dir(configs.ApkPath)
	apkBasenameWithExt := path.Base(configs.ApkPath)
	apkExt := filepath.Ext(apkBasenameWithExt)
	apkBasename := strings.TrimSuffix(apkBasenameWithExt, apkExt)

	//
	// Download keystore
	keystorePath := ""
	if strings.HasPrefix(configs.KeystoreURL, "file://") {
		pth := strings.TrimPrefix(configs.KeystoreURL, "file://")
		var err error
		keystorePath, err = pathutil.AbsPath(pth)
		if err != nil {
			log.Error("Failed to expand path (%s), error: %s", pth, err)
			os.Exit(1)
		}
	} else {
		log.Info("Download keystore")
		keystorePath = path.Join(tmpDir, "keystore.jks")
		if err := download(configs.KeystoreURL, keystorePath); err != nil {
			log.Error("Failed to download keystore, error: %s", err)
			os.Exit(1)
		}
	}
	log.Detail("using keystore at: %s", keystorePath)
	fmt.Println("")

	keystore, err := keystore.NewHelper(keystorePath, configs.KeystorePassword, configs.KeystoreAlias)
	if err != nil {
		log.Error("Failed to create keystore helper, error: %s", err)
		os.Exit(1)
	}

	//
	// Find Android tools
	androidHome := os.Getenv("ANDROID_HOME")
	log.Detail("android_home: %s", androidHome)

	latestBuildToolsDir, err := latestBuildToolsDir(androidHome)
	if err != nil {
		log.Error("failed to find latest build-tools")
		os.Exit(1)
	}
	log.Detail("build_tools: %s", latestBuildToolsDir)

	aapt := filepath.Join(latestBuildToolsDir, "aapt")
	if exist, err := pathutil.IsPathExists(aapt); err != nil {
		log.Error("Failed to find aapt path, error: %s", err)
		os.Exit(1)
	} else if !exist {
		log.Error("aapt not found at: %s", aapt)
		os.Exit(1)
	}
	log.Detail("aapt: %s", aapt)

	zipalign := filepath.Join(latestBuildToolsDir, "zipalign")
	if exist, err := pathutil.IsPathExists(zipalign); err != nil {
		log.Error("Failed to find zipalign path, error: %s", err)
		os.Exit(1)
	} else if !exist {
		log.Error("zipalign not found at: %s", zipalign)
		os.Exit(1)
	}
	log.Detail("zipalign: %s", zipalign)
	fmt.Println()

	//
	// Resign apk
	unsignedAPKPth := filepath.Join(tmpDir, "unsigned.apk")
	cmdex.CopyFile(configs.ApkPath, unsignedAPKPth)

	log.Info("Unsign APK if signed")
	if err := unsignAPK(aapt, unsignedAPKPth); err != nil {
		log.Error("Failed to unsign APK, error: %s", err)
		os.Exit(1)
	}
	log.Done("unsiged")
	fmt.Println()

	unalignedAPKPth := filepath.Join(tmpDir, "unaligned.apk")
	log.Info("Sign APK")
	if err := keystore.SignAPK(unsignedAPKPth, unalignedAPKPth, configs.PrivateKeyPassword); err != nil {
		log.Error("Failed to sign APK, error: %s", err)
		os.Exit(1)
	}
	log.Done("signed")
	fmt.Println()

	log.Info("Verify APK")
	if err := keystore.VerifyAPK(unalignedAPKPth); err != nil {
		log.Error("Failed to verify APK, error: %s", err)
		os.Exit(1)
	}
	log.Done("verified")
	fmt.Println()

	log.Info("Zipalign APK")
	signedAPKPth := filepath.Join(apkDir, apkBasename+"-bitrise-signed"+apkExt)
	if err := zipalignAPK(zipalign, unalignedAPKPth, signedAPKPth); err != nil {
		log.Error("Failed to zipalign APK, error: %s", err)
		os.Exit(1)
	}
	log.Done("zipaligned")
	fmt.Println()

	// Exporting signed ipa
	fmt.Println("")
	log.Done("Signed APK created at: %s", signedAPKPth)
	if err := exportEnvironmentWithEnvman("BITRISE_SIGNED_APK_PATH", signedAPKPth); err != nil {
		log.Warn("Failed to export APK, error: %s", err)
	}
	if err := exportEnvironmentWithEnvman("BITRISE_APK_PATH", signedAPKPth); err != nil {
		log.Warn("Failed to export APK, error: %s", err)
	}
}
