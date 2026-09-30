//go:build !windows

package radikron

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestConcatAACFilesRemovesConsumedBatches(t *testing.T) {
	tempDir := t.TempDir()
	binDir := t.TempDir()
	ffmpeg := filepath.Join(binDir, "ffmpeg")
	if err := os.WriteFile(ffmpeg, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	files := make([]string, 205)
	for i := range files {
		files[i] = filepath.Join(tempDir, fmt.Sprintf("chunk-%03d.aac", i))
		if err := os.WriteFile(files[i], []byte("chunk"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	if err := concatAACFiles(context.Background(), files, tempDir, filepath.Join(tempDir, "output.aac")); err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("consumed input %q remains (stat err: %v)", file, err)
		}
	}
}
