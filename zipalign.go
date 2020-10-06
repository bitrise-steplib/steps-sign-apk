package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-utils/log"
)

func zipalignBuildArtifact(zipalignConfig *zipalignConfiguration, artifactPath, dstPath string) (string, error) {
	aligned, err := zipalignConfig.checkAlignment(artifactPath)
	if err != nil {
		return "", err
	}
	if aligned {
		return artifactPath, nil
	}

	err = zipalignConfig.zipalignArtifact(artifactPath, dstPath)
	if err != nil {
		return "", err
	}

	return dstPath, err
}

func zipAlignArtifact(zipalignPath, unalignedBuildArtifactPth string, buildArtifactDir string, buildArtifactBasename string, artifactExt string, fullPathExt string, outputName string, pageAlignConfig pageAlignStatus) (string, error) {
	log.Infof("Zipalign Build Artifact")
	signedArtifactName := fmt.Sprintf("%s-bitrise-%s%s", buildArtifactBasename, fullPathExt, artifactExt)
	if artifactName := fmt.Sprintf("%s%s", outputName, artifactExt); outputName != "" {
		log.Printf("- Exporting (%s) as: %s", signedArtifactName, artifactName)
		signedArtifactName = artifactName
	}
	fullPath := filepath.Join(buildArtifactDir, signedArtifactName)

	isPageAligned := pageAlignConfig == pageAlignYes
	// Only care about .so memory page alignment for APKs
	if !strings.EqualFold(artifactExt, ".aab") && pageAlignConfig == pageAlignAuto {
		extractNativeLibs, err := parseAPKextractNativeLibs(unalignedBuildArtifactPth)
		if err != nil {
			log.Warnf("Failed to parse APK manifest to read extractNativeLibs attribute: %s", err)
			isPageAligned = true
		} else {
			isPageAligned = !extractNativeLibs
		}
	}

	return zipalignBuildArtifact(newZipalignConfiguration(zipalignPath, isPageAligned),
		unalignedBuildArtifactPth, fullPath)
}
