package fs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// skippedNames are never part of a skill's content hash.
var skippedNames = map[string]bool{".git": true, "node_modules": true, ".DS_Store": true}

// HashDir returns a deterministic sha256 over the sorted relative paths and
// contents of every regular file under dir. If dir itself is a symlink the
// target's tree is hashed. Symlinks inside the tree contribute their link
// target string instead of following it.
func HashDir(dir string) (string, error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", dir, err)
	}
	type entry struct{ rel, link string }
	var entries []entry
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if skippedNames[d.Name()] && p != root {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		e := entry{rel: filepath.ToSlash(rel)}
		if d.Type()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			e.link = target
		}
		entries = append(entries, e)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk %s: %w", root, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].rel < entries[j].rel })

	h := sha256.New()
	for _, e := range entries {
		fmt.Fprintf(h, "%s\x00", e.rel)
		if e.link != "" {
			fmt.Fprintf(h, "link:%s\x00", e.link)
			continue
		}
		f, err := os.Open(filepath.Join(root, filepath.FromSlash(e.rel)))
		if err != nil {
			return "", err
		}
		n, err := io.Copy(h, f)
		f.Close()
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "\x00%d\x00", n)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
