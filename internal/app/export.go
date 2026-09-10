package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/domain"
)

// DefaultPluginVersion is used when an export does not set one.
const DefaultPluginVersion = "0.1.0"

// pluginNameRE matches the plugin.json "name" charset used by Claude
// marketplaces (kebab-case).
var pluginNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// ExportOptions describes a Claude plugin export (PLAN.md section 4 step 3).
type ExportOptions struct {
	OutDir      string
	Name        string
	Version     string
	Description string
	Skills      []string // vault entry names
}

// ExportResult reports where the plugin was written.
type ExportResult struct {
	Dir      string   `json:"dir"`
	Manifest string   `json:"manifest"`
	Skills   []string `json:"skills"`
}

// pluginManifest is .claude-plugin/plugin.json, matching the files Claude
// Code writes to ~/.claude/plugins/cache/<marketplace>/<plugin>/<version>/.
type pluginManifest struct {
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Description string        `json:"description,omitempty"`
	Author      *pluginAuthor `json:"author,omitempty"`
}

type pluginAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

// ExportPlugin writes <outDir>/<name>/.claude-plugin/plugin.json, a
// byte-for-byte copy of every chosen vault skill under skills/<name>/ and
// a README.md listing them. The plugin directory must not exist yet.
func (s *Service) ExportPlugin(ctx context.Context, opts ExportOptions) (ExportResult, error) {
	res := ExportResult{Skills: []string{}}
	name := strings.TrimSpace(opts.Name)
	if !pluginNameRE.MatchString(name) {
		return res, fmt.Errorf("plugin name %q: use lowercase letters, digits and dashes: %w", opts.Name, domain.ErrRejected)
	}
	if len(opts.Skills) == 0 {
		return res, errors.New("export needs at least one vault skill")
	}
	if strings.TrimSpace(opts.OutDir) == "" {
		return res, errors.New("export needs an output directory")
	}
	outDir, err := filepath.Abs(opts.OutDir)
	if err != nil {
		return res, err
	}
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version = DefaultPluginVersion
	}
	var entries []domain.VaultEntry
	seen := map[string]bool{}
	for _, sk := range opts.Skills {
		sk = strings.TrimSpace(sk)
		if seen[sk] {
			continue
		}
		seen[sk] = true
		e, err := s.vault.GetVault(ctx, sk)
		if err != nil {
			return res, fmt.Errorf("vault entry %q: %w", sk, err)
		}
		if _, err := os.Stat(filepath.Join(e.Path, skillfs.SkillFile)); err != nil {
			return res, fmt.Errorf("vault entry %q has no skill directory", sk)
		}
		entries = append(entries, e)
	}
	dir := filepath.Join(outDir, name)
	if _, err := os.Lstat(dir); err == nil {
		return res, fmt.Errorf("%s %w; remove it or pick another name", dir, domain.ErrExists)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".claude-plugin"), 0o755); err != nil {
		return res, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(dir)
		}
	}()
	manifest := pluginManifest{Name: name, Version: version, Description: strings.TrimSpace(opts.Description)}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return res, err
	}
	res.Manifest = filepath.Join(dir, ".claude-plugin", "plugin.json")
	if err := os.WriteFile(res.Manifest, append(b, '\n'), 0o644); err != nil {
		return res, err
	}
	var readme strings.Builder
	fmt.Fprintf(&readme, "# %s\n\n", name)
	if manifest.Description != "" {
		fmt.Fprintf(&readme, "%s\n\n", manifest.Description)
	}
	fmt.Fprintf(&readme, "Claude Code plugin exported by skillman (version %s).\n\n## Skills\n\n", version)
	for _, e := range entries {
		dest := filepath.Join(dir, "skills", e.Name)
		if err := skillfs.CopyTree(e.Path, dest); err != nil {
			return res, fmt.Errorf("copy %s: %w", e.Name, err)
		}
		hash, err := skillfs.HashDir(dest)
		if err != nil {
			return res, err
		}
		if e.ContentHash != "" && hash != e.ContentHash {
			return res, fmt.Errorf("copy of %s does not match the vault (hash %s vs %s)", e.Name, shortHash(hash), shortHash(e.ContentHash))
		}
		desc := ""
		if info, err := s.inspector.Inspect(dest); err == nil {
			desc = info.Description
		}
		if desc != "" {
			fmt.Fprintf(&readme, "- `%s`: %s\n", e.Name, desc)
		} else {
			fmt.Fprintf(&readme, "- `%s`\n", e.Name)
		}
		res.Skills = append(res.Skills, e.Name)
	}
	fmt.Fprintf(&readme, "\nInstall with `claude plugin install %s` from a marketplace that lists this directory, or copy it into a marketplace repository.\n", name)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme.String()), 0o644); err != nil {
		return res, err
	}
	res.Dir = dir
	ok = true
	return res, nil
}
