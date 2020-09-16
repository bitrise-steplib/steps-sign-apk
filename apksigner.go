package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
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

func createPCKSCmdSlice(configuration *PKCSSignatureConfiguration) ([]string, error) {

	if configuration == nil {
		return []string{}, errors.New("Invalid PKCS Configuration")
	}

	cmdSlice := []string{
		"--provider-class",
		"sun.security.pkcs11.SunPKCS11",
		"--ks-type",
		"PKCS11",
		"--ks",
		"NONE",
		"--provider-arg",
		configuration.providerArgument,
	}

	return cmdSlice, nil
}

func createCertificateCmdSlice(configuration *CertificateSignatureConfiguration) ([]string, error) {

	if configuration == nil {
		return []string{}, errors.New("Invalid Certificate Configuration")
	}

	cmdSlice := []string{
		"--key",
		configuration.keyPath,
		"--cert",
		configuration.certificatePath,
	}
	return cmdSlice, nil
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

	var signatureSlice []string = []string{}
	var err error = nil

	switch configuration.signatureType {
	case KeystoreSignatureType:
		signatureSlice, err = createKeystoreCmdSlice(configuration.keystoreConfiguration)
	case CertificateSignatureType:
		signatureSlice, err = createCertificateCmdSlice(configuration.certiciateConfiguration)
	case PKCSSignatureType:
		signatureSlice, err = createPCKSCmdSlice(configuration.pcksConfiguartion)
	default:
		err = fmt.Errorf("Invalid signature type: %s", configuration.signatureType)
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

// SignBuidlArtifact buildArtifactPth
// This signs the provided APK, stripping out any pre-existing signatures. Signing
// is performed using one or more signers, each represented by an asymmetric key
// pair and a corresponding certificate.
//
// - buildArtifactPth: The path to the unsigned APK
// - destBuildArtifactPth: Path were the signed APK will be stored
func (configuration SignatureConfiguration) SignBuidlArtifact(buildArtifactPth string, destBuildArtifactPth string) error {
	cmdSlice, err := configuration.createSignCmd(buildArtifactPth, destBuildArtifactPth)

	prinatableCmd := command.PrintableCommandArgs(false, secureSignCmd(cmdSlice))
	log.Printf("=> %s", prinatableCmd)

	out, err := executeForOutput(cmdSlice)
	if err != nil {
		return properError(err, out)
	}

	return err
}

// VerifyBuildArtifact buildArtifactPth
// This checks whether the provided APK will verify on Android. By default, this
// checks whether the APK will verify on all Android platform versions supported
// by the APK (as declared using minSdkVersion in AndroidManifest.xml).
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

	prinatableCmd := command.PrintableCommandArgs(false, cmdSlice)
	log.Printf("=> %s", prinatableCmd)

	out, err := executeForOutput(cmdSlice)
	if err != nil {
		return properError(err, out)
	}

	return nil
}

func executeForOutput(cmdSlice []string) (string, error) {
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

func properError(err error, out string) error {
	if errorutil.IsExitStatusError(err) {
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
