package keystore

// https://github.com/calabash/calabash-android/blob/6bb3d9ac9eadf353dc7573c28a957e88e6669f67/ruby-gem/lib/calabash-android/helpers.rb

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"io"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
)

const jarsigner = "/usr/bin/jarsigner"

// Helper ...
type Helper struct {
	keystorePth        string
	keystorePassword   string
	alias              string
	signatureAlgorithm string
}

// Execute ...
func Execute(cmdSlice []string) error {
	prinatableCmd := command.PrintableCommandArgs(false, cmdSlice)
	log.Printf("=> %s", prinatableCmd)
	fmt.Println("")

	cmd, err := command.NewFromSlice(cmdSlice)
	if err != nil {
		return fmt.Errorf("Failed to create command, error: %s", err)
	}

	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	log.Printf(out)
	return err
}

// ExecuteForOutput ...
func ExecuteForOutput(cmdSlice []string) (string, error) {
	cmd, err := command.NewFromSlice(cmdSlice)
	if err != nil {
		return "", fmt.Errorf("Failed to create command, error: %s", err)
	}

	var errBuf, outputBuf bytes.Buffer
	writer := io.MultiWriter(&outputBuf, &errBuf)
	cmd.SetStderr(writer)

	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("%s\n%s\n%s", outputBuf.String(), errBuf.String(), err)
	}

	return outputBuf.String(), err
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

	out, err := ExecuteForOutput(cmdSlice)
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

	prinatableCmd := command.PrintableCommandArgs(false, secureSignCmd(cmdSlice))
	log.Printf("=> %s", prinatableCmd)

	out, err := ExecuteForOutput(cmdSlice)
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

	prinatableCmd := command.PrintableCommandArgs(false, cmdSlice)
	log.Printf("=> %s", prinatableCmd)

	out, err := ExecuteForOutput(cmdSlice)
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
