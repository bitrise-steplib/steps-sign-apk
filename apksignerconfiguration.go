package main

import (
	"fmt"
	"os"

	"github.com/bitrise-tools/go-android/sdk"
)

// SignatureType ..
type SignatureType string

// SignatureType values
const (
	KeystoreSignatureType    SignatureType = "keystore"
	PKCSSignatureType                      = "pcks"
	CertificateSignatureType               = "certificate"
)

// CertificateSignatureConfiguration ...
type CertificateSignatureConfiguration struct {
	keyPath         string
	certificatePath string
}

// PKCSSignatureConfiguration ...
type PKCSSignatureConfiguration struct {
	providerArgument string
}

// KeystoreSignatureConfiguration ..
type KeystoreSignatureConfiguration struct {
	keystorePth      string
	keystorePassword string
	aliasPassword    string
	alias            string
}

// SignatureConfiguration ...
type SignatureConfiguration struct {
	apkSigner               string
	signerScheme            string
	debuggablePermitted     string
	signatureType           SignatureType
	certiciateConfiguration *CertificateSignatureConfiguration
	pcksConfiguartion       *PKCSSignatureConfiguration
	keystoreConfiguration   *KeystoreSignatureConfiguration
}

func buildAPKSignerPath() (string, error) {

	androidHome := os.Getenv("ANDROID_HOME")
	androidSDK, err := sdk.New(androidHome)
	signer, err := androidSDK.LatestBuildToolPath("apksigner")

	if err != nil {
		return "", fmt.Errorf("failed to create sdk model, error: %s", err)
	}

	return signer, err
}

// NewKeystoreSignatureConfiguration ...
func NewKeystoreSignatureConfiguration(keystore string, keystorePassword string, alias string, aliasPassword string, debuggablePermitted string, signerScheme string) (SignatureConfiguration, error) {

	apkSigner, err := buildAPKSignerPath()

	if err != nil {
		return SignatureConfiguration{}, err
	}

	keystoreConfig := KeystoreSignatureConfiguration{
		keystorePth:      keystore,
		keystorePassword: keystorePassword,
		alias:            alias,
		aliasPassword:    aliasPassword,
	}

	return SignatureConfiguration{
		apkSigner:             apkSigner,
		debuggablePermitted:   debuggablePermitted,
		signerScheme:          signerScheme,
		signatureType:         KeystoreSignatureType,
		keystoreConfiguration: &keystoreConfig,
	}, nil
}

// NewCertificateConfiguration ...
func NewCertificateConfiguration(key string, certificate string, debuggablePermitted string, signerScheme string) (SignatureConfiguration, error) {

	apkSigner, err := buildAPKSignerPath()

	if err != nil {
		return SignatureConfiguration{}, err
	}

	certificateConfig := CertificateSignatureConfiguration{
		certificatePath: certificate,
		keyPath:         key,
	}

	return SignatureConfiguration{
		apkSigner:               apkSigner,
		debuggablePermitted:     debuggablePermitted,
		signerScheme:            signerScheme,
		signatureType:           CertificateSignatureType,
		certiciateConfiguration: &certificateConfig,
	}, nil
}

// NewPKCSConfiguration ...
func NewPKCSConfiguration(providerArgument string, debuggablePermitted string, signerScheme string) (SignatureConfiguration, error) {
	apkSigner, err := buildAPKSignerPath()

	if err != nil {
		return SignatureConfiguration{}, err
	}

	pkcsConfiguration := PKCSSignatureConfiguration{
		providerArgument: providerArgument,
	}

	return SignatureConfiguration{
		apkSigner:           apkSigner,
		debuggablePermitted: debuggablePermitted,
		signerScheme:        signerScheme,
		signatureType:       PKCSSignatureType,
		pcksConfiguartion:   &pkcsConfiguration,
	}, nil
}
