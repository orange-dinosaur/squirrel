package upgrade

import (
	"os"
	"path/filepath"
)

// Replace swaps the file at path, or the file a symlink at path points to, for an executable
// holding data. The new file is written next to the old one and renamed over it, so a failure
// leaves the old file whole. A permission error matches fs.ErrPermission.
func Replace(path string, data []byte) (err error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(real), ".squirrel-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), real)
}
