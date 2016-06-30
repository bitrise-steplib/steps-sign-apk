package keystore

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateSignCmd(t *testing.T) {
	t.Log("signature algorithm: SHA1withRSA")
	{
		apkPth := "android.apk"
		destApkPth := "android-signed.apk"
		keystorePath := "keystore.jks"
		keystoreAlias := "alias"
		keystorePassword := "pass"
		signatureAlgorithm := "SHA1withRSA"

		cmdSlice, err := createSignCmd(apkPth, destApkPth, keystorePath, keystoreAlias, keystorePassword, signatureAlgorithm)
		require.NoError(t, err)
		require.Equal(t, 15, len(cmdSlice))

		actual := strings.Join(cmdSlice, " ")
		expected := jarsigner + " -sigfile CERT -sigalg SHA1withRSA -digestalg SHA1 -keystore keystore.jks -storepass pass -signedjar android-signed.apk android.apk alias"
		require.Equal(t, expected, actual)
	}

	t.Log("signature algorithm: MD5withRSA")
	{
		apkPth := "android.apk"
		destApkPth := "android-signed.apk"
		keystorePath := "keystore.jks"
		keystoreAlias := "alias"
		keystorePassword := "pass"
		signatureAlgorithm := "MD5withRSA"

		cmdSlice, err := createSignCmd(apkPth, destApkPth, keystorePath, keystoreAlias, keystorePassword, signatureAlgorithm)
		require.NoError(t, err)
		require.Equal(t, 15, len(cmdSlice))

		actual := strings.Join(cmdSlice, " ")
		expected := jarsigner + " -sigfile CERT -sigalg SHA1withRSA -digestalg SHA1 -keystore keystore.jks -storepass pass -signedjar android-signed.apk android.apk alias"
		require.Equal(t, expected, actual)
	}

	t.Log("signature algorithm: MD5withRSAandMGF1")
	{
		apkPth := "android.apk"
		destApkPth := "android-signed.apk"
		keystorePath := "keystore.jks"
		keystoreAlias := "alias"
		keystorePassword := "pass"
		signatureAlgorithm := "MD5withRSAandMGF1"

		cmdSlice, err := createSignCmd(apkPth, destApkPth, keystorePath, keystoreAlias, keystorePassword, signatureAlgorithm)
		require.NoError(t, err)
		require.Equal(t, 15, len(cmdSlice))

		actual := strings.Join(cmdSlice, " ")
		expected := jarsigner + " -sigfile CERT -sigalg SHA1withRSA -digestalg SHA1 -keystore keystore.jks -storepass pass -signedjar android-signed.apk android.apk alias"
		require.Equal(t, expected, actual)
	}
}

func TestFindSignatureAlgorithm(t *testing.T) {
	keystoreData := `Alias name: MyAndroidKey
Creation date: Jun 2, 2016
Entry type: PrivateKeyEntry
Certificate chain length: 1
Certificate[1]:
Owner: CN=Bitrise, OU=Mobile Development, O=MyCompany, L=Budapest, ST=Pest, C=HU
Issuer: CN=Bitrise, OU=Mobile Development, O=MyCompany, L=Budapest, ST=Pest, C=HU
Serial number: 5750111
Valid from: Thu Jun 02 19:56:20 CEST 2016 until: Mon May 27 19:56:20 CEST 2041
Certificate fingerprints:
	 MD5:  CA:30:61:CB:AD:70:03:73:C7:FD:91:A4:9C:FB:92:F9
	 SHA1: 66:C3:60:5B:B8:0B:B0:2C:AE:C5:54:72:B6:B2:D6:18:99:FB:70:9F
	 Signature algorithm name: SHA1withRSA
	 Version: 3
`
	signatureAlgorithm, err := findSignatureAlgorithm(keystoreData)
	require.NoError(t, err)
	require.Equal(t, "SHA1withRSA", signatureAlgorithm)
}
