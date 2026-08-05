package main

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-android/v2/gradle/artifactmap"
	"github.com/bitrise-io/go-utils/log"
)

// artifactRename records that signing produced `signed` from `original`.
type artifactRename struct {
	original string
	signed   string
}

// updateArtifactMap keeps the variant-keyed artifact map (exported by the
// Android build steps as BITRISE_ANDROID_ARTIFACT_MAP_PATH) pointing at the
// current files: signing writes a renamed copy (e.g. *-bitrise-signed.aab)
// next to the input artifact, which would otherwise break the map's
// file-identity pairing for every later consumer (e.g. Google Play deploy).
//
// Never fatal: signing has already succeeded, so an absent or unreadable map
// only logs.
func updateArtifactMap(mapPath string, renames []artifactRename) {
	if mapPath == "" {
		return
	}
	m, err := artifactmap.Read(mapPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Debugf("No artifact map at %s, nothing to update", mapPath)
		} else {
			log.Warnf("Artifact map at %s is unreadable, not updating it: %s", mapPath, err)
		}
		return
	}

	fmt.Println()
	log.Infof("Updating the artifact map")
	updated := false
	for _, rename := range renames {
		oldName := filepath.Base(rename.original)
		newName := filepath.Base(rename.signed)
		if oldName == newName {
			continue
		}
		if m.ReplaceFile(oldName, newName) {
			log.Printf("Artifact map: %s -> %s", oldName, newName)
			updated = true
		} else {
			log.Debugf("The artifact map does not reference %s, skipping", oldName)
		}
	}
	if !updated {
		log.Printf("The artifact map does not reference any of the signed artifacts, leaving it unchanged")
		return
	}

	if err := artifactmap.Write(mapPath, m); err != nil {
		log.Warnf("Failed to update the artifact map: %s", err)
		return
	}
	log.Donef("The artifact map now references the signed artifacts: %s", mapPath)
	// Print the document so pairing can be debugged from the build log alone.
	if doc, err := artifactmap.Marshal(m); err == nil {
		log.Printf("Artifact map contents:")
		fmt.Println(strings.TrimSuffix(string(doc), "\n"))
	}
}
