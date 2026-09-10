package fs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CopyTree copies src to dest without following symlinks. dest must not exist.
func CopyTree(src, dest string) error {
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("copy to %s: %w", dest, fs.ErrExist)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := copyTree(src, dest); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	return nil
}

// Symlink creates path pointing at target, creating parent dirs. path must
// not exist. When relative is true the link is written relative to the
// directory containing path.
func Symlink(target, path string, relative bool) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("link %s: %w", path, fs.ErrExist)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if relative {
		rel, err := filepath.Rel(filepath.Dir(path), target)
		if err != nil {
			return err
		}
		target = rel
	}
	return os.Symlink(target, path)
}

// RemoveEntry deletes path: a symlink is unlinked without touching its
// target, a directory is removed recursively. A missing path is not an error.
func RemoveEntry(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return os.Remove(path)
	}
	return os.RemoveAll(path)
}

// ReplaceDir atomically-ish swaps dest with src: dest is renamed aside,
// src is moved into place and the old tree is removed. On failure the old
// tree is restored. Both must be on the same filesystem for rename to work;
// a cross-device move falls back to copy.
func ReplaceDir(src, dest string) error {
	old := dest + ".old"
	_ = os.RemoveAll(old)
	hadOld := false
	if _, err := os.Lstat(dest); err == nil {
		if err := os.Rename(dest, old); err != nil {
			return fmt.Errorf("move %s aside: %w", dest, err)
		}
		hadOld = true
	}
	if err := move(src, dest); err != nil {
		if hadOld {
			_ = os.Rename(old, dest)
		}
		return err
	}
	if hadOld {
		return os.RemoveAll(old)
	}
	return nil
}

// IsWithin reports whether path is dir or lies under dir after cleaning
// both. Symlinks are not resolved.
func IsWithin(path, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
