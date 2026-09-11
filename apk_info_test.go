package main

import (
	"context"
	"os"
	"os/exec"
	"path"
	"reflect"
	"testing"
)

func Test_parseAPKextractNativeLibs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("setup: failed to create temp dir, error: %s", err)
	}

	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("failed to remove temp dir, error: %s", err)
		}
	}()

	cloneCmd := exec.CommandContext(context.Background(), "git", "clone", "https://github.com/bitrise-io/sample-artifacts.git", tmpDir)
	if err := cloneCmd.Run(); err != nil {
		t.Fatalf("setup: failed to clone test artifact repo, error: %s", err)
	}

	tests := []struct {
		name    string
		apkPath string
		want    bool
		wantErr bool
	}{
		{
			name:    "",
			apkPath: path.Join(tmpDir, "apks", "app-debug.apk"),
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAPKextractNativeLibs(tt.apkPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseAPKInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseAPKInfo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
