package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrettyAPKBasename(t *testing.T) {
	require.Equal(t, "app", prettyAPKBasename("app-unsigned.apk"))
	require.Equal(t, "app-signed", prettyAPKBasename("app-signed.apk"))
	require.Equal(t, "app-debug", prettyAPKBasename("app-debug.apk"))
	require.Equal(t, "app-release", prettyAPKBasename("app-release.apk"))
}

func TestFilterMETAFiles(t *testing.T) {
	t.Log("finds files in META-INF folder")
	{
		fileList := []string{
			"META-INF/MANIFEST.MF",
			"META-INF/CERT.SF",
			"META-INF/CERT.RSA",
			"AndroidManifest.xml",
			"res/anim/abc_fade_in.xml",
			"res/anim/abc_fade_out.xml",
			"res/anim/abc_grow_fade_in_from_bottom.xml",
		}

		metaFiles := filterMETAFiles(fileList)
		require.Equal(t, 3, len(metaFiles))
		require.Equal(t, "META-INF/MANIFEST.MF", metaFiles[0])
		require.Equal(t, "META-INF/CERT.SF", metaFiles[1])
		require.Equal(t, "META-INF/CERT.RSA", metaFiles[2])
	}
}

func TestFilterSigningFiles(t *testing.T) {
	t.Log("finds .mf files")
	{
		fileList := []string{
			"META-INF/MANIFEST.MF",
			"res/anim/abc_fade_in.xml",
		}

		metaFiles := filterSigningFiles(fileList)
		require.Equal(t, 1, len(metaFiles))
		require.Equal(t, "META-INF/MANIFEST.MF", metaFiles[0])
	}

	t.Log("finds .rsa files")
	{
		fileList := []string{
			"META-INF/MANIFEST.RSA",
			"res/anim/abc_fade_in.xml",
		}

		metaFiles := filterSigningFiles(fileList)
		require.Equal(t, 1, len(metaFiles))
		require.Equal(t, "META-INF/MANIFEST.RSA", metaFiles[0])
	}

	t.Log("finds .dsa files")
	{
		fileList := []string{
			"META-INF/MANIFEST.DSA",
			"res/anim/abc_fade_in.xml",
		}

		metaFiles := filterSigningFiles(fileList)
		require.Equal(t, 1, len(metaFiles))
		require.Equal(t, "META-INF/MANIFEST.DSA", metaFiles[0])
	}

	t.Log("finds .ec files")
	{
		fileList := []string{
			"META-INF/MANIFEST.EC",
			"res/anim/abc_fade_in.xml",
		}

		metaFiles := filterSigningFiles(fileList)
		require.Equal(t, 1, len(metaFiles))
		require.Equal(t, "META-INF/MANIFEST.EC", metaFiles[0])
	}

	t.Log("finds .sf files")
	{
		fileList := []string{
			"META-INF/MANIFEST.SF",
			"res/anim/abc_fade_in.xml",
		}

		metaFiles := filterSigningFiles(fileList)
		require.Equal(t, 1, len(metaFiles))
		require.Equal(t, "META-INF/MANIFEST.SF", metaFiles[0])
	}
}
