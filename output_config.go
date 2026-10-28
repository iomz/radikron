package radikron

import (
	"fmt"
	"os"
	"path/filepath"
)

// OutputConfig describes the target directory and file for a recording.
type OutputConfig struct {
	DirFullPath  string
	FileBaseName string
	FileFormat   string
}

func (c *OutputConfig) SetupDir() error {
	return os.MkdirAll(c.DirFullPath, DirPermissions)
}

func (c *OutputConfig) AudioFormat() string { return c.FileFormat }

// AbsPath returns where a new recording is written.
func (c *OutputConfig) AbsPath() string {
	return c.pathWithExt(outputFileExt(c.FileFormat))
}

// ExistingPath returns the recording already on disk, including AAC recordings
// saved as raw ADTS .aac files before AAC moved into an .m4a container.
func (c *OutputConfig) ExistingPath() (string, bool) {
	paths := []string{c.AbsPath()}
	if c.FileFormat == AudioFormatAAC {
		paths = append(paths, c.pathWithExt(AudioFormatAAC))
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

func (c *OutputConfig) IsExist() bool {
	_, ok := c.ExistingPath()
	return ok
}

func (c *OutputConfig) pathWithExt(ext string) string {
	return filepath.Join(c.DirFullPath, fmt.Sprintf("%s.%s", c.FileBaseName, ext))
}

// outputFileExt maps an audio format to its file extension. AAC goes in an MP4
// container because raw ADTS has no duration index, so players guess the
// length of HE-AAC recordings wrong.
func outputFileExt(fileFormat string) string {
	if fileFormat == AudioFormatAAC {
		return FileExtM4A
	}
	return fileFormat
}
