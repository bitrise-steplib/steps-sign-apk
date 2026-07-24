package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/bitrise-io/go-utils/v2/command"
)

func createSignerSchemeCmd(signerScheme string) string {
	switch signerScheme {
	case "automatic":
		return ""
	case "v2":
		return "--v2-signing-enabled"
	case "v3":
		return "--v3-signing-enabled"
	case "v4":
		return "--v4-signing-enabled"
	default:
		return ""
	}
}

func createKeystoreCmdSlice(configuration *KeystoreSignatureConfiguration) ([]string, error) {
	if configuration == nil {
		return []string{}, errors.New("Invalid Keystore Configuration")
	}

	cmdSlice := []string{
		"--ks",
		configuration.keystorePth,
		"--ks-pass",
		"pass:" + configuration.keystorePassword,
		"--ks-key-alias",
		configuration.alias,
	}

	if configuration.aliasPassword != "" {
		cmdSlice = append(cmdSlice, "--key-pass", "pass:"+configuration.aliasPassword)
	}

	return cmdSlice, nil
}

func (configuration SignatureConfiguration) createSignCmd(buildArtifactPth string, destBuildArtifactPth string) ([]string, error) {
	var signatureSlice []string
	var err error

	switch configuration.signatureType {
	case KeystoreSignatureType:
		signatureSlice, err = createKeystoreCmdSlice(configuration.keystoreConfiguration)
	default:
		err = fmt.Errorf("invalid signature type: %s", configuration.signatureType)
	}

	if err != nil {
		return nil, err
	}

	cmdSlice := []string{
		configuration.apkSigner,
		"sign",
		"--in",
		buildArtifactPth,
		"--out",
		destBuildArtifactPth,
		"--debuggable-apk-permitted",
		configuration.debuggablePermitted,
	}

	scheme := createSignerSchemeCmd(configuration.signerScheme)

	if scheme != "" {
		cmdSlice = append(cmdSlice, scheme)
	}

	cmdSlice = append(cmdSlice, signatureSlice...)

	return cmdSlice, nil
}

// SignBuildArtifact signs the provided APK, stripping out any pre-existing signatures.
// Signing is performed using one or more signers, each represented by an asymmetric
// key pair and a corresponding certificate.
//
// - buildArtifactPth: The path to the unsigned APK
// - destBuildArtifactPth: Path were the signed APK will be stored
func (configuration SignatureConfiguration) SignBuildArtifact(buildArtifactPth string, destBuildArtifactPth string) error {
	cmdSlice, err := configuration.createSignCmd(buildArtifactPth, destBuildArtifactPth)
	if err != nil {
		return fmt.Errorf("failed to create signing command from signing configuration: %v", err)
	}

	configuration.logger.Printf("=> %s", printableCommandArgs(configuration.cmdFactory, secureSignCmd(cmdSlice)))

	out, err := executeForOutput(configuration.cmdFactory, cmdSlice)
	if err != nil {
		return properError(err, out)
	}

	return err
}

// VerifyBuildArtifact checks whether the provided APK will verify on Android.
// By default this checks all Android platform versions supported by the APK
// (as declared using minSdkVersion in AndroidManifest.xml).
//
// - buildArtifactPth: The path of the signed APK
func (configuration SignatureConfiguration) VerifyBuildArtifact(buildArtifactPth string) error {
	cmdSlice := []string{
		configuration.apkSigner,
		"verify",
		"--verbose",
		"--in",
		buildArtifactPth,
	}

	configuration.logger.Printf("=> %s", printableCommandArgs(configuration.cmdFactory, cmdSlice))

	out, err := executeForOutput(configuration.cmdFactory, cmdSlice)
	if err != nil {
		return properError(err, out)
	}

	return nil
}

func executeForOutput(cmdFactory command.Factory, cmdSlice []string) (string, error) {
	if len(cmdSlice) == 0 {
		return "", fmt.Errorf("empty command")
	}

	var outputBuf bytes.Buffer
	writer := io.MultiWriter(&outputBuf)
	cmd := cmdFactory.Create(cmdSlice[0], cmdSlice[1:], &command.Opts{
		Stdout: writer,
		Stderr: writer,
	})

	err := cmd.Run()
	if err != nil {
		err = fmt.Errorf("%s\n%s", outputBuf.String(), err)
	}

	return outputBuf.String(), err
}

// printableCommandArgs shell-escapes cmdSlice via a throwaway v2 command.
func printableCommandArgs(cmdFactory command.Factory, cmdSlice []string) string {
	if len(cmdSlice) == 0 {
		return ""
	}

	cmd := cmdFactory.Create(cmdSlice[0], cmdSlice[1:], nil)

	return cmd.PrintableCommandArgs()
}

func properError(err error, out string) error {
	var exitErr *command.ExitStatusError
	if errors.As(err, &exitErr) {
		return errors.New(out)
	}

	return err
}

func secureSignCmd(cmdSlice []string) []string {
	securedCmdSlice := []string{}
	secureNextParam := false
	for _, param := range cmdSlice {
		if secureNextParam {
			param = "***"
		}

		secureNextParam = (param == "--ks-pass" || param == "--key-pass")
		securedCmdSlice = append(securedCmdSlice, param)
	}

	return securedCmdSlice
}
