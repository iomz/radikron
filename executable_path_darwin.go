//go:build darwin

package radikron

import (
	"os"
	"path/filepath"
	"strings"
)

// initFFmpegPath adds common Homebrew locations after the inherited PATH.
// Keeping them last preserves shell-configured ffmpeg precedence.
func initFFmpegPath() {
	_ = os.Setenv("PATH", withHomebrewPaths(os.Getenv("PATH")))
}

func withHomebrewPaths(path string) string {
	for _, dir := range []string{"/opt/homebrew/bin", "/usr/local/bin"} {
		if !pathContains(path, dir) {
			path = strings.Trim(path, string(os.PathListSeparator))
			if path != "" {
				path += string(os.PathListSeparator)
			}
			path += dir
		}
	}
	return path
}

func pathContains(path, dir string) bool {
	for _, entry := range filepath.SplitList(path) {
		if entry == dir {
			return true
		}
	}
	return false
}
