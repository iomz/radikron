package update

import (
	"fmt"
	"os"
	"path/filepath"
)

// replaceExecutable atomically swaps the file at exePath (after resolving
// symlinks) for binary, keeping its permission bits.
//
// The new binary is written to a temporary file in the same directory and
// renamed into place, so the target is never half-written and gets a new
// inode; overwriting a signed macOS binary in place can get it killed.
// Windows cannot replace a running executable but can rename it, so there the
// old file moves to "<exe>.old" first and is deleted on the next update.
func replaceExecutable(exePath string, binary []byte, goos string) (err error) {
	target, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("update: resolve executable: %w", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("update: stat executable: %w", err)
	}
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".radikron-update-*")
	if err != nil {
		return fmt.Errorf("update: cannot write next to %s; rerun with permission to modify it: %w", target, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err = tmp.Write(binary); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("update: write new executable: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("update: sync new executable: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("update: close new executable: %w", err)
	}
	if err = os.Chmod(tmpName, info.Mode().Perm()); err != nil {
		return fmt.Errorf("update: set executable mode: %w", err)
	}

	if goos != "windows" {
		if err = os.Rename(tmpName, target); err != nil {
			return fmt.Errorf("update: install new executable: %w", err)
		}
		return nil
	}

	old := target + ".old"
	_ = os.Remove(old) // left over from the previous update, if any
	if err = os.Rename(target, old); err != nil {
		return fmt.Errorf("update: move running executable aside: %w", err)
	}
	if err = os.Rename(tmpName, target); err != nil {
		if restoreErr := os.Rename(old, target); restoreErr != nil {
			return fmt.Errorf("update: install new executable: %w (restoring %s also failed: %v)", err, target, restoreErr)
		}
		return fmt.Errorf("update: install new executable: %w", err)
	}
	_ = os.Remove(old) // fails while the old binary is still running
	return nil
}
