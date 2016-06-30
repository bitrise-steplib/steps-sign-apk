package keystore

// https://github.com/calabash/calabash-android/blob/6bb3d9ac9eadf353dc7573c28a957e88e6669f67/ruby-gem/lib/calabash-android/helpers.rb

import (
	"bufio"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/bitrise-io/go-utils/cmdex"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/pathutil"
	log "github.com/bitrise-io/sign-apk/logger"
	"github.com/bitrise-io/sign-apk/run"
)

const jarsigner = "/usr/bin/jarsigner"

// KeystoreModel ...
type KeystoreModel struct {
	Path               string
	Alias              string
	Password           string
	SignatureAlgorithm string
}

// NewKeystoreModel ...
func NewKeystoreModel(pth, password, alias string) (KeystoreModel, error) {
	if exist, err := pathutil.IsPathExists(pth); err != nil {
		return KeystoreModel{}, err
	} else if !exist {
		return KeystoreModel{}, fmt.Errorf("keystore not exist at: %s", pth)
	}

	cmdSlice := []string{
		"keytool",
		"-list",
		"-v",
		"-alias",
		alias,
		"-keystore",
		pth,
		"-storepass",
		password,
		"-J-Dfile.encoding=utf-8",
		"-J-Duser.language=en-US",
	}

	keystoreData, err := run.ExecuteForOutput(cmdSlice)
	if err != nil {
		if errorutil.IsExitStatusError(err) {
			return KeystoreModel{}, errors.New(keystoreData)
		}
		return KeystoreModel{}, err
	}

	if keystoreData == "" {
		return KeystoreModel{}, fmt.Errorf("failed to read keystore, maybe alias (%s) or password (%s) is not correct", alias, "****")
	}

	signatureAlgorithm, err := findSignatureAlgorithm(keystoreData)
	if err != nil {
		return KeystoreModel{}, err
	}
	if signatureAlgorithm == "" {
		return KeystoreModel{}, errors.New("failed to find signature algorithm")
	}

	return KeystoreModel{
		Path:               pth,
		Alias:              alias,
		Password:           password,
		SignatureAlgorithm: signatureAlgorithm,
	}, nil
}

// SignAPK ...
func (keystore KeystoreModel) SignAPK(apkPth, destApkPth, password string) error {
	if exist, err := pathutil.IsPathExists(apkPth); err != nil {
		return err
	} else if !exist {
		return fmt.Errorf("APK not exist at: %s", apkPth)
	}

	cmdSlice, err := createSignCmd(
		apkPth,
		destApkPth,
		keystore.Path,
		keystore.Alias,
		keystore.Password,
		keystore.SignatureAlgorithm,
	)
	if err != nil {
		return err
	}

	prinatableCmd := cmdex.PrintableCommandArgs(false, cmdSlice)
	log.Details("=> %s", prinatableCmd)

	out, err := run.ExecuteForOutput(cmdSlice)
	if err != nil {
		if errorutil.IsExitStatusError(err) {
			return errors.New(out)
		}
		return err
	}
	if !strings.Contains(out, "jar signed.") {
		return errors.New(out)
	}
	return nil
}

// VerifyAPK ...
func (keystore KeystoreModel) VerifyAPK(apkPth string) error {
	cmdSlice := []string{
		jarsigner,
		"-verify",
		"-verbose",
		"-certs",
		apkPth,
	}

	prinatableCmd := cmdex.PrintableCommandArgs(false, cmdSlice)
	log.Details("=> %s", prinatableCmd)

	out, err := run.ExecuteForOutput(cmdSlice)
	if err != nil {
		if errorutil.IsExitStatusError(err) {
			return errors.New(out)
		}
		return err
	}
	if !strings.Contains(out, "jar verified.") {
		return errors.New(out)
	}
	return nil
}

func createSignCmd(apkPth, destApkPth, keystorePath, keystoreAlias, keystorePassword, signatureAlgorithm string) ([]string, error) {
	split := strings.Split(signatureAlgorithm, "with")
	if len(split) != 2 {
		return []string{}, fmt.Errorf("failed to parse signature algorithm: %s", signatureAlgorithm)
	}
	split = strings.Split(split[1], "and")

	signingAlgorithm := "SHA1with" + split[0]
	digestAlgorithm := "SHA1"

	return []string{
		jarsigner,
		"-sigfile",
		"CERT",

		"-sigalg",
		signingAlgorithm,
		"-digestalg",
		digestAlgorithm,

		"-keystore",
		keystorePath,
		"-storepass",
		keystorePassword,

		"-signedjar",
		destApkPth,
		apkPth,
		keystoreAlias,
	}, nil
}

func findSignatureAlgorithm(keystoreData string) (string, error) {
	exp := regexp.MustCompile(`Signature algorithm name: (.*)`)

	scanner := bufio.NewScanner(strings.NewReader(keystoreData))
	for scanner.Scan() {
		matches := exp.FindStringSubmatch(scanner.Text())
		if len(matches) > 1 {
			return matches[1], nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", nil
}
