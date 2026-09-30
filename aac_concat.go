package radikron

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func concatAACFilesFromList(ctx context.Context, resourcesDir string) (string, error) {
	entries, err := os.ReadDir(resourcesDir)
	if err != nil {
		return "", err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, filepath.Join(resourcesDir, entry.Name()))
		}
	}
	output := filepath.Join(resourcesDir, "concated.aac")
	if err := concatAACFiles(ctx, files, resourcesDir, output); err != nil {
		return "", err
	}
	return output, nil
}

func concatAACFiles(ctx context.Context, files []string, tempDir, output string) error {
	if len(files) > 100 {
		first := files[:100]
		rest := files[100:]
		tmp, err := os.CreateTemp(tempDir, "tmp-concatenated-*.aac")
		if err != nil {
			return err
		}
		tmpPath := tmp.Name()
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		defer os.Remove(tmpPath)
		if err := concatAACFiles(ctx, first, tempDir, tmpPath); err != nil {
			return err
		}
		return concatAACFiles(ctx, append([]string{tmpPath}, rest...), tempDir, output)
	}
	list, err := os.CreateTemp(tempDir, "aac-resources-*")
	if err != nil {
		return err
	}
	listPath := list.Name()
	defer os.Remove(listPath)
	for _, file := range files {
		concatPath := filepath.ToSlash(file)
		concatPath = strings.ReplaceAll(concatPath, "'", "'\\''")
		if _, err := fmt.Fprintf(list, "file '%s'\n", concatPath); err != nil {
			_ = list.Close()
			return err
		}
	}
	if err := list.Close(); err != nil {
		return err
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, ffmpeg, "-f", "concat", "-safe", "0", "-y", "-i", listPath, "-c", "copy", output)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg AAC concatenation failed: %w", err)
	}
	return nil
}
