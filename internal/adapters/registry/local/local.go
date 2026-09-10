// Package local treats a filesystem path as a skill source: a skill or
// plugin directory, a single rule/command file, a SKILL.md path or a local
// .zip/.tar.gz archive.
package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/adapters/registry/archive"
	"github.com/melvicsosa/skillman/internal/domain"
)

// ID is the registry identifier.
const ID = "local"

// Source implements domain.Registry for local paths.
type Source struct {
	// Home expands a leading "~"; defaults to os.UserHomeDir.
	Home func() (string, error)
}

var _ domain.Registry = Source{}

// ID returns "local".
func (Source) ID() string { return ID }

// Search is not supported.
func (Source) Search(context.Context, string) ([]domain.RegistryResult, error) {
	return nil, fmt.Errorf("local paths: search %w", domain.ErrUnsupported)
}

// Looks reports whether ref looks like a local path (it starts with ., /
// or ~, or exists on disk).
func Looks(ref string) bool {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "../") || strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "~") || ref == "." {
		return true
	}
	if strings.Contains(ref, "://") || strings.Contains(ref, ":") {
		return false
	}
	_, err := os.Stat(ref)
	return err == nil
}

// Resolve expands "~" and makes ref absolute.
func (s Source) Resolve(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "~" || strings.HasPrefix(ref, "~/") {
		home := s.Home
		if home == nil {
			home = os.UserHomeDir
		}
		h, err := home()
		if err != nil {
			return "", err
		}
		ref = filepath.Join(h, strings.TrimPrefix(ref, "~"))
	}
	abs, err := filepath.Abs(ref)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// Fetch copies the path into a temp dir so conversion never touches the
// original. Archives are extracted; a SKILL.md path selects its directory.
func (s Source) Fetch(_ context.Context, ref string) (domain.FetchResult, error) {
	abs, err := s.Resolve(ref)
	if err != nil {
		return domain.FetchResult{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return domain.FetchResult{}, fmt.Errorf("local path: %w", err)
	}
	if !info.IsDir() && filepath.Base(abs) == skillfs.SkillFile {
		abs = filepath.Dir(abs)
		info, err = os.Stat(abs)
		if err != nil {
			return domain.FetchResult{}, err
		}
	}
	tmp, err := os.MkdirTemp("", "skillman-local-*")
	if err != nil {
		return domain.FetchResult{}, err
	}
	cleanup := func() { os.RemoveAll(tmp) }
	src := domain.VaultSource{Type: domain.SourceLocal, Ref: abs, URL: "file://" + abs}
	name := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	var dir string
	switch {
	case info.IsDir():
		dir = filepath.Join(tmp, filepath.Base(abs))
		if err := skillfs.CopyTree(abs, dir); err != nil {
			cleanup()
			return domain.FetchResult{}, err
		}
	case IsArchive(abs):
		f, err := os.Open(abs)
		if err != nil {
			cleanup()
			return domain.FetchResult{}, err
		}
		defer f.Close()
		dir, name, err = ExtractInto(f, abs, tmp)
		if err != nil {
			cleanup()
			return domain.FetchResult{}, err
		}
	default:
		dir = filepath.Join(tmp, filepath.Base(abs))
		if err := skillfs.CopyTree(abs, dir); err != nil {
			cleanup()
			return domain.FetchResult{}, err
		}
	}
	return domain.FetchResult{Dir: dir, Name: name, Source: src, Cleanup: cleanup}, nil
}

// IsArchive reports whether name ends with a supported archive extension.
func IsArchive(name string) bool {
	l := strings.ToLower(name)
	return strings.HasSuffix(l, ".zip") || strings.HasSuffix(l, ".tar.gz") || strings.HasSuffix(l, ".tgz")
}

// ExtractInto extracts the archive read from r (named name) under tmp and
// returns the directory to convert: the single top-level directory when
// the archive has one, otherwise the extraction root named after the
// archive. The suggested skill name is the directory's base name.
func ExtractInto(r interface{ Read([]byte) (int, error) }, name, tmp string) (dir, skillName string, err error) {
	base := strings.ToLower(filepath.Base(name))
	stem := filepath.Base(name)
	for _, ext := range []string{".tar.gz", ".tgz", ".zip"} {
		if strings.HasSuffix(base, ext) {
			stem = stem[:len(stem)-len(ext)]
			break
		}
	}
	root := filepath.Join(tmp, "extract")
	var res archive.Result
	if strings.HasSuffix(base, ".zip") {
		res, err = archive.ExtractZip(r, root, archive.Options{})
	} else {
		res, err = archive.ExtractTarGz(r, root, archive.Options{})
	}
	if err != nil {
		return "", "", err
	}
	if res.Files == 0 {
		return "", "", errors.New("archive is empty")
	}
	if res.TopDir != "" {
		if entries, err := os.ReadDir(root); err == nil && len(entries) == 1 && entries[0].IsDir() {
			return filepath.Join(root, res.TopDir), res.TopDir, nil
		}
	}
	dir = filepath.Join(tmp, stem)
	if err := os.Rename(root, dir); err != nil {
		return "", "", err
	}
	return dir, stem, nil
}

// Matches reports whether ref looks like a local path.
func (Source) Matches(ref string) bool { return Looks(ref) }
