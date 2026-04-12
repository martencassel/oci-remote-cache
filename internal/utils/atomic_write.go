package utils

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// AtomicWriteFile streams from r → temp file → fsync → rename.
// No buffering. Safe for multi‑GB blobs.
func AtomicWriteFile(path string, r io.Reader, perm fs.FileMode) error {
	dir := filepath.Dir(path)

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Create temp file in same directory
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	// Stream data into temp file
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}

	// fsync file
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}

	// close before rename
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	// atomic rename
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}

	// fsync directory for durability
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}

	// apply permissions
	return os.Chmod(path, perm)
}
