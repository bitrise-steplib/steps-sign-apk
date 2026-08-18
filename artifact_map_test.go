package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bitrise-io/go-android/v2/gradle/artifactmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestMap(t *testing.T) string {
	t.Helper()
	m, _ := artifactmap.Build(
		[]artifactmap.File{{DeployPath: "/deploy/app-demo-release.apk", SourcePath: "/src/app/build/outputs/apk/demo/release/app-demo-release.apk"}},
		[]artifactmap.File{{DeployPath: "/deploy/app-demo-release.aab", SourcePath: "/src/app/build/outputs/bundle/demoRelease/app-demo-release.aab"}},
		nil,
		[]artifactmap.File{{DeployPath: "/deploy/mapping.txt", SourcePath: "/src/app/build/outputs/mapping/demoRelease/mapping.txt"}},
	)
	mapPath := filepath.Join(t.TempDir(), artifactmap.DefaultFileName)
	require.NoError(t, artifactmap.Write(mapPath, m))
	return mapPath
}

func Test_updateArtifactMap_RenamesSignedArtifacts(t *testing.T) {
	mapPath := writeTestMap(t)

	updateArtifactMap(mapPath, []artifactRename{
		{original: "/deploy/app-demo-release.aab", signed: "/deploy/app-demo-release-bitrise-signed.aab"},
		{original: "/deploy/not-in-map.apk", signed: "/deploy/not-in-map-bitrise-signed.apk"},
	})

	m, err := artifactmap.Read(mapPath)
	require.NoError(t, err)
	entry := m.Modules["app"]["demoRelease"]
	assert.Equal(t, []string{"app-demo-release-bitrise-signed.aab"}, entry.AAB)
	assert.Equal(t, []string{"app-demo-release.apk"}, entry.APK, "unsigned APK reference must stay")
	assert.Equal(t, "mapping.txt", entry.Mapping, "mapping reference must stay")
}

func Test_updateArtifactMap_NoReferencedArtifacts_LeavesFileUnchanged(t *testing.T) {
	mapPath := writeTestMap(t)
	before, err := os.ReadFile(mapPath)
	require.NoError(t, err)

	updateArtifactMap(mapPath, []artifactRename{
		{original: "/deploy/other.apk", signed: "/deploy/other-bitrise-signed.apk"},
	})

	after, err := os.ReadFile(mapPath)
	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func Test_updateArtifactMap_MissingOrGarbageMap_IsTolerated(t *testing.T) {
	// missing file: no panic, no file created
	missing := filepath.Join(t.TempDir(), artifactmap.DefaultFileName)
	updateArtifactMap(missing, []artifactRename{{original: "/a.apk", signed: "/a-signed.apk"}})
	_, err := os.Stat(missing)
	assert.True(t, os.IsNotExist(err))

	// garbage file: left untouched
	garbagePath := filepath.Join(t.TempDir(), artifactmap.DefaultFileName)
	require.NoError(t, os.WriteFile(garbagePath, []byte("not json"), 0600))
	updateArtifactMap(garbagePath, []artifactRename{{original: "/a.apk", signed: "/a-signed.apk"}})
	content, err := os.ReadFile(garbagePath)
	require.NoError(t, err)
	assert.Equal(t, "not json", string(content))

	// empty input path: no-op
	updateArtifactMap("", []artifactRename{{original: "/a.apk", signed: "/a-signed.apk"}})
}
