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

func (c *OutputConfig) AbsPath() string {
	return filepath.Join(c.DirFullPath, fmt.Sprintf("%s.%s", c.FileBaseName, c.FileFormat))
}

func (c *OutputConfig) IsExist() bool {
	_, err := os.Stat(c.AbsPath())
	return err == nil
}
