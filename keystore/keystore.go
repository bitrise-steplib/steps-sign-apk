package keystore

// https://github.com/calabash/calabash-android/blob/6bb3d9ac9eadf353dc7573c28a957e88e6669f67/ruby-gem/lib/calabash-android/helpers.rb

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

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
	signerScheme       string
}

// Execute ...
func Execute(cmdSlice []string) error {
	cmd, err := command.NewFromSlice(cmdSlice)
	if err != nil {
		return fmt.Errorf("Failed to create command, error: %s", err)
	}

	log.Printf("=> %s\n", cmd.PrintableCommandArgs())

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

	var outputBuf bytes.Buffer
	writer := io.MultiWriter(&outputBuf)
	cmd.SetStderr(writer)
	cmd.SetStdout(writer)

	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("%s\n%s", outputBuf.String(), err)
	}

	return outputBuf.String(), err
}

// NewHelper ...
func NewHelper(keystorePth, keystorePassword, alias string, signerScheme string) (Helper, error) {
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
		signerScheme:       signerScheme,
	}, nil
}

func createSignerSchemeCmd(signerScheme string) string {
	switch signerScheme {
	case "automatic":
		return ""
	case "v2":
		return "--v2-signing-enabled true"
	case "v3":
		return "--v3-signing-enabled true"
	case "v4":
		return "--v4-signing-enabled true"
	default:
		return ""
	}
}

func (helper Helper) createSignCmd(buildArtifactPth, destBuildArtifactPth, privateKeyPassword string) ([]string, error) {
	split := strings.Split(helper.signatureAlgorithm, "with")
	if len(split) != 2 {
		return []string{}, fmt.Errorf("failed to parse signature algorithm: %s", helper.signatureAlgorithm)
	}
	split = strings.Split(split[1], "and")

	signingAlgorithm := "SHA1with" + split[0]
	digestAlgorithm := "SHA1"
	schema := createSignerSchemeCmd(helper.signerScheme)

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

		schema,
	}

	if privateKeyPassword != "" {
		cmdSlice = append(cmdSlice, "-keypass", privateKeyPassword)
	}

	cmdSlice = append(cmdSlice, "-signedjar", destBuildArtifactPth, buildArtifactPth, helper.alias)

	return cmdSlice, nil
}

// SignBuildArtifact ...
func (helper Helper) SignBuildArtifact(buildArtifactPth, destBuildArtifactPth, privateKeyPassword string) error {
	if exist, err := pathutil.IsPathExists(buildArtifactPth); err != nil {
		return err
	} else if !exist {
		return fmt.Errorf("Build Artifact not exist at: %s", buildArtifactPth)
	}

	cmdSlice, err := helper.createSignCmd(buildArtifactPth, destBuildArtifactPth, privateKeyPassword)
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

// VerifyBuildArtifact ...
func (helper Helper) VerifyBuildArtifact(buildArtifactPth string) error {
	cmdSlice := []string{
		jarsigner,
		"-verify",
		"-verbose",
		"-certs",
		buildArtifactPth,
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
	scanner := bufio.NewScanner(strings.NewReader(keystoreData))

	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "Signature algorithm name: ") {
			split := strings.Split(line, "Signature algorithm name: ")

			if len(split) < 2 {
				return "", fmt.Errorf("failed to expand signature algorithm from: %s", line)
			}

			alg := split[1]
			split = strings.Split(alg, " ")

			if len(split) > 1 {
				log.Warnf("🚨 Signature algorithm name contains unnecessary postfix: %s", alg)
				log.Printf("Trimmed signature algorithm name: %s", split[0])

				alg = split[0]
			}
			return alg, nil
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
