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

// Helper ...
type Helper struct {
	keystorePth        string
	keystorePassword   string
	alias              string
	signatureAlgorithm string
}

// NewHelper ...
func NewHelper(keystorePth, keystorePassword, alias string) (Helper, error) {
	if exist, err := pathutil.IsPathExists(keystorePth); err != nil {
		return Helper{}, err
	} else if !exist {
		return Helper{}, fmt.Errorf("keystore not exist at: %s", keystorePth)
	}

	cmdSlice := []string{
		"keytool",
		"-list",
		"-v",

		"-keystore",
		keystorePth,
		"-storepass",
		keystorePassword,

		"-alias",
		alias,

		"-J-Dfile.encoding=utf-8",
		"-J-Duser.language=en-US",
	}

	out, err := run.ExecuteForOutput(cmdSlice)
	if err != nil {
		return Helper{}, properError(err, out)
	}
	if out == "" {
		return Helper{}, fmt.Errorf("failed to read keystore, maybe alias (%s) or password (%s) is not correct", alias, "****")
	}

	signatureAlgorithm, err := findSignatureAlgorithm(out)
	if err != nil {
		return Helper{}, err
	}
	if signatureAlgorithm == "" {
		return Helper{}, errors.New("failed to find signature algorithm")
	}

	return Helper{
		keystorePth:        keystorePth,
		keystorePassword:   keystorePassword,
		alias:              alias,
		signatureAlgorithm: signatureAlgorithm,
	}, nil
}

func (helper Helper) createSignCmd(apkPth, destApkPth, privateKeyPassword string) ([]string, error) {
	split := strings.Split(helper.signatureAlgorithm, "with")
	if len(split) != 2 {
		return []string{}, fmt.Errorf("failed to parse signature algorithm: %s", helper.signatureAlgorithm)
	}
	split = strings.Split(split[1], "and")

	signingAlgorithm := "SHA1with" + split[0]
	digestAlgorithm := "SHA1"

	cmdSlice := []string{
		jarsigner,
		"-sigfile",
		"CERT",

		"-sigalg",
		signingAlgorithm,
		"-digestalg",
		digestAlgorithm,

		"-keystore",
		helper.keystorePth,
		"-storepass",
		helper.keystorePassword,
	}

	if privateKeyPassword != "" {
		cmdSlice = append(cmdSlice, "-keypass", privateKeyPassword)
	}

	cmdSlice = append(cmdSlice, "-signedjar", destApkPth, apkPth, helper.alias)

	return cmdSlice, nil
}

// SignAPK ...
func (helper Helper) SignAPK(apkPth, destApkPth, privateKeyPassword string) error {
	if exist, err := pathutil.IsPathExists(apkPth); err != nil {
		return err
	} else if !exist {
		return fmt.Errorf("APK not exist at: %s", apkPth)
	}

	cmdSlice, err := helper.createSignCmd(apkPth, destApkPth, privateKeyPassword)
	if err != nil {
		return err
	}

	prinatableCmd := cmdex.PrintableCommandArgs(false, secureSignCmd(cmdSlice))
	log.Details("=> %s", prinatableCmd)

	out, err := run.ExecuteForOutput(cmdSlice)
	if err != nil {
		return properError(err, out)
	}
	if !strings.Contains(out, "jar signed.") {
		return errors.New(out)
	}
	return nil
}

// VerifyAPK ...
func (helper Helper) VerifyAPK(apkPth string) error {
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
		return properError(err, out)
	}
	if !strings.Contains(out, "jar verified.") {
		return errors.New(out)
	}
	return nil
}

func properError(err error, out string) error {
	if errorutil.IsExitStatusError(err) {
		return errors.New(out)
	}
	return err
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

func secureSignCmd(cmdSlice []string) []string {
	securedCmdSlice := []string{}
	secureNextParam := false
	for _, param := range cmdSlice {
		if secureNextParam {
			param = "***"
		}

		secureNextParam = (param == "-storepass" || param == "-keypass")
		securedCmdSlice = append(securedCmdSlice, param)
	}
	return securedCmdSlice
}
