package main

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/avast/apkparser"
)

type manifest struct {
	XMLName     xml.Name `xml:"manifest"`
	Application application
}

type application struct {
	XMLName           xml.Name `xml:"application"`
	ExtractNativeLibs bool     `xml:"extractNativeLibs,attr"` // defaults to false
}

func parseAPKextractNativeLibs(apkPath string) (bool, error) {
	var manifestContent bytes.Buffer
	enc := xml.NewEncoder(&manifestContent)
	enc.Indent("", "\t")

	zipErr, resErr, manErr := apkparser.ParseApk(apkPath, enc)
	if zipErr != nil {
		return false, fmt.Errorf("failed to unzip the APK: %s", zipErr)
	}
	if resErr != nil {
		return false, fmt.Errorf("failed to parse resources: %s", resErr)
	}
	if manErr != nil {
		return false, fmt.Errorf("failed to parse AndroidManifest.xml: %s", manErr)
	}

	var manifest manifest
	if err := xml.Unmarshal(manifestContent.Bytes(), &manifest); err != nil {
		return false, fmt.Errorf("failed to unmarshal AndroidManifest.xml: %s", err)
	}

	return manifest.Application.ExtractNativeLibs, nil
}
