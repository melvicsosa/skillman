package fs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// Mover moves skill directories in and out of a quarantine directory.
// Symlinks are moved as links; their targets are never touched.
type Mover struct{}

// Disable moves src into quarantineDir, keeping its base name, and returns
// the destination. It refuses to overwrite an existing destination.
func (Mover) Disable(src, quarantineDir string) (string, error) {
	if _, err := os.Lstat(src); err != nil {
		return "", fmt.Errorf("disable %s: %w", src, err)
	}
	if err := os.MkdirAll(quarantineDir, 0o755); err != nil {
		return "", fmt.Errorf("create quarantine dir: %w", err)
	}
	dest := filepath.Join(quarantineDir, filepath.Base(src))
	if err := move(src, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// Enable moves a quarantined entry back to originalPath.
func (Mover) Enable(quarantined, originalPath string) error {
	if _, err := os.Lstat(quarantined); err != nil {
		return fmt.Errorf("enable %s: %w", quarantined, err)
	}
	if err := os.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
		return fmt.Errorf("create parent of %s: %w", originalPath, err)
	}
	return move(quarantined, originalPath)
}

// move renames src to dest, refusing to overwrite. When the rename crosses
// devices it falls back to copy + remove. Symlinks are moved as links.
func move(src, dest string) error {
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("move %s: destination %s already exists", src, dest)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", dest, err)
	}
	err := os.Rename(src, dest)
	if err == nil {
		return nil
	}
	if !isCrossDevice(err) {
		return fmt.Errorf("move %s to %s: %w", src, dest, err)
	}
	if err := copyTree(src, dest); err != nil {
		_ = os.RemoveAll(dest)
		return fmt.Errorf("copy %s to %s: %w", src, dest, err)
	}
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("remove %s after copy: %w", src, err)
	}
	return nil
}

func isCrossDevice(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && errno == syscall.EXDEV
}

// copyTree copies src to dest without following symlinks.
func copyTree(src, dest string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dest)
	case info.IsDir():
		if err := os.MkdirAll(dest, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dest, e.Name())); err != nil {
				return err
			}
		}
		return nil
	default:
		return copyFile(src, dest, info.Mode().Perm())
	}
}

func copyFile(src, dest string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
