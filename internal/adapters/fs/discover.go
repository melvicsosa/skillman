package fs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Discovered is one candidate skill directory found by DiscoverSkills.
type Discovered struct {
	Name       string
	Path       string // absolute path of the entry inside the skills dir
	IsSymlink  bool
	LinkTarget string // absolute resolved target when IsSymlink
	Dangling   bool   // IsSymlink and the target does not exist
}

// DiscoverSkills lists the immediate children of dir that are directories
// (or symlinks to directories) containing SKILL.md. Dangling symlinks are
// returned with Dangling set so callers can report them. Entries named
// "_shared", names starting with "." and non-directories are skipped. A
// missing dir yields no results and no error.
func DiscoverSkills(dir string) ([]Discovered, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var out []Discovered
	for _, e := range entries {
		name := e.Name()
		if name == "_shared" || strings.HasPrefix(name, ".") {
			continue
		}
		p := filepath.Join(dir, name)
		d := Discovered{Name: name, Path: p}
		if e.Type()&fs.ModeSymlink != 0 {
			d.IsSymlink = true
			raw, err := os.Readlink(p)
			if err != nil {
				return nil, fmt.Errorf("readlink %s: %w", p, err)
			}
			if !filepath.IsAbs(raw) {
				raw = filepath.Join(dir, raw)
			}
			d.LinkTarget = filepath.Clean(raw)
			if resolved, err := filepath.EvalSymlinks(p); err == nil {
				d.LinkTarget = resolved
			}
			info, err := os.Stat(p)
			if err != nil {
				d.Dangling = true
				out = append(out, d)
				continue
			}
			if !info.IsDir() {
				continue
			}
		} else if !e.IsDir() {
			continue
		}
		if info, err := os.Stat(filepath.Join(p, SkillFile)); err != nil || info.IsDir() {
			continue
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
