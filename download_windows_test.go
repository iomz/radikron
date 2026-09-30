//go:build windows

package radikron

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetRadikronPathMapsMacUserDownloadsPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	got, err := GetRadikronPath("/Users/iomz/Downloads/radiko")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(homeDir, "Downloads", "radiko")
	if got != want {
		t.Fatalf("GetRadikronPath() = %q, want %q", got, want)
	}
}

func TestGetRadikronPathRejectsUnsupportedPosixRootOnWindows(t *testing.T) {
	if _, err := GetRadikronPath("/tmp/radiko"); err == nil {
		t.Fatal("GetRadikronPath() accepted POSIX /tmp path on Windows")
	}
}
