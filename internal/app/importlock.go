package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/domain"
)

// LockFileName is the vercel `skills` CLI lockfile inside ~/.agents.
const LockFileName = ".skill-lock.json"

// lockFile mirrors ~/.agents/.skill-lock.json (version 3, observed
// 2026-09-10): a map of skill name to source facts.
type lockFile struct {
	Version int                  `json:"version"`
	Skills  map[string]lockEntry `json:"skills"`
}

type lockEntry struct {
	Source          string `json:"source"`     // "owner/repo"
	SourceType      string `json:"sourceType"` // "github"
	SourceURL       string `json:"sourceUrl"`  // "https://github.com/owner/repo.git"
	SkillPath       string `json:"skillPath"`  // "skills/<name>/SKILL.md"
	SkillFolderHash string `json:"skillFolderHash"`
	PluginName      string `json:"pluginName"`
	InstalledAt     string `json:"installedAt"`
	UpdatedAt       string `json:"updatedAt"`
}

// LockSkip explains why a lock entry was not imported.
type LockSkip struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// LockImportResult reports an ImportLock run.
type LockImportResult struct {
	LockFile string     `json:"lockFile"`
	Imported []string   `json:"imported"`
	Skipped  []LockSkip `json:"skipped"`
}

// LockPath returns the default lockfile location, derived from the shared
// ~/.agents/skills dir so SKILLMAN_HOME is honoured.
func (s *Service) LockPath() string {
	for _, a := range s.agents.Agents() {
		for _, d := range a.GlobalDirs {
			if filepath.Base(filepath.Dir(d)) == ".agents" {
				return filepath.Join(filepath.Dir(d), LockFileName)
			}
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agents", LockFileName)
}

// ImportLock registers every skill of the lockfile into the vault by
// copying its installed directory (~/.agents/skills/<name>) and recording
// "lock" provenance. Installed dirs are left as they are.
func (s *Service) ImportLock(ctx context.Context, lockPath string) (LockImportResult, error) {
	if lockPath == "" {
		lockPath = s.LockPath()
	}
	res := LockImportResult{LockFile: lockPath, Imported: []string{}, Skipped: []LockSkip{}}
	b, err := os.ReadFile(lockPath)
	if err != nil {
		return res, fmt.Errorf("read lockfile: %w", err)
	}
	var lock lockFile
	if err := json.Unmarshal(b, &lock); err != nil {
		return res, fmt.Errorf("%s: %w", lockPath, err)
	}
	skillsDir := filepath.Join(filepath.Dir(lockPath), "skills")
	names := make([]string, 0, len(lock.Skills))
	for n := range lock.Skills {
		names = append(names, n)
	}
	sortStrings(names)
	for _, name := range names {
		le := lock.Skills[name]
		if _, err := s.vault.GetVault(ctx, name); err == nil {
			res.Skipped = append(res.Skipped, LockSkip{Name: name, Reason: "already in vault"})
			continue
		} else if !errors.Is(err, domain.ErrNotFound) {
			return res, err
		}
		installed := filepath.Join(skillsDir, name)
		if _, err := os.Stat(filepath.Join(installed, skillfs.SkillFile)); err != nil {
			res.Skipped = append(res.Skipped, LockSkip{Name: name, Reason: "not installed at " + installed})
			continue
		}
		staging, err := os.MkdirTemp("", "skillman-convert-*")
		if err != nil {
			return res, err
		}
		report, err := s.converter.Convert(installed, staging, name)
		if err != nil {
			os.RemoveAll(staging)
			res.Skipped = append(res.Skipped, LockSkip{Name: name, Reason: err.Error()})
			continue
		}
		subpath := path.Dir(filepath.ToSlash(le.SkillPath))
		if subpath == "." {
			subpath = ""
		}
		source := domain.VaultSource{
			Type: domain.SourceLock, Ref: le.Source, URL: le.SourceURL, Subpath: subpath, Commit: le.SkillFolderHash,
		}
		if subpath != "" {
			source.Ref = le.Source + "/" + subpath
		}
		sourceHash, err := skillfs.HashDir(installed)
		if err != nil {
			os.RemoveAll(staging)
			return res, err
		}
		installedAt, _ := time.Parse(time.RFC3339Nano, le.InstalledAt)
		if _, err := s.storeInVault(ctx, report.Skills[0], source, sourceHash, installedAt); err != nil {
			os.RemoveAll(staging)
			return res, err
		}
		os.RemoveAll(staging)
		res.Imported = append(res.Imported, name)
	}
	if len(res.Imported) > 0 {
		if _, err := s.Scan(ctx, ScanOptions{}); err != nil {
			return res, err
		}
	}
	return res, nil
}

func sortStrings(s []string) { sort.Strings(s) }
