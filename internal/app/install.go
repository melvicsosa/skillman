package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/melvicsosa/skillman/internal/adapters/convert"
	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/domain"
)

// ClaudeCodeID is the agent that only reads .claude/skills in projects.
const ClaudeCodeID domain.AgentID = "claude-code"

// sharedProjectDir is the cross-agent per-project skills dir (PLAN.md 1.2).
const sharedProjectDir = ".agents/skills"

// claudeProjectDir is where Claude Code reads per-project skills.
const claudeProjectDir = ".claude/skills"

// InstallOptions selects where a vault entry is installed.
type InstallOptions struct {
	Agents []domain.AgentID
	All    bool   // every enabled agent that can receive installs
	Root   string // project root; "" installs globally
	Copy   bool   // copy instead of symlink
	// Command also writes a .claude/commands/<name>.md stub for Claude Code.
	Command bool
}

// LinkResult is one path written (or found) by an install.
type LinkResult struct {
	Agent    string `json:"agent"`
	Scope    string `json:"scope"`
	Path     string `json:"path"`
	Copied   bool   `json:"copied"`
	Existing bool   `json:"existing"` // already installed from this entry
}

// InstallFromVault links vault entry name into the requested agents. It
// refuses to overwrite a skill dir that is not this entry (ErrExists) and
// rescans afterwards so the cache shows the new links.
func (s *Service) InstallFromVault(ctx context.Context, name string, opts InstallOptions) ([]LinkResult, error) {
	entry, err := s.vault.GetVault(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("vault entry %q: %w", name, err)
	}
	targets, err := s.installTargets(ctx, opts)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, errors.New("no target agents: pass --to <agent> or --all")
	}
	root := ""
	if opts.Root != "" {
		root, err = cleanRoot(opts.Root)
		if err != nil {
			return nil, err
		}
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			return nil, fmt.Errorf("project root %s is not a directory", root)
		}
	}
	var (
		results []LinkResult
		done    = map[string]bool{}
	)
	for _, agent := range targets {
		primary, secondary := s.linkPaths(agent, root, entry.Name)
		scope := string(domain.GlobalScope)
		if root != "" {
			scope = string(domain.ProjectScope(root))
		}
		if !done[primary] {
			done[primary] = true
			r, err := s.placeSkill(entry, primary, opts.Copy)
			if err != nil {
				return results, err
			}
			r.Agent, r.Scope = string(agent.ID), scope
			results = append(results, r)
		}
		if secondary != "" && !done[secondary] {
			done[secondary] = true
			r, err := s.placeLink(entry, primary, secondary)
			if err != nil {
				return results, err
			}
			r.Agent, r.Scope = string(agent.ID), scope
			results = append(results, r)
		}
		if opts.Command && agent.ID == ClaudeCodeID {
			if err := s.writeCommandStub(agent, root, entry, primary); err != nil {
				return results, err
			}
		}
	}
	projects := map[string]bool{}
	if root != "" {
		projects[root] = true
	}
	if err := s.rescan(ctx, projects); err != nil {
		return results, err
	}
	return results, nil
}

// installTargets resolves the agents an install applies to.
func (s *Service) installTargets(ctx context.Context, opts InstallOptions) ([]domain.Agent, error) {
	all, err := s.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[domain.AgentID]domain.Agent{}
	for _, a := range all {
		byID[a.ID] = a.Agent
	}
	var out []domain.Agent
	if opts.All {
		for _, a := range all {
			if a.Enabled && a.ID != pluginsAgentID {
				out = append(out, a.Agent)
			}
		}
		return out, nil
	}
	for _, id := range opts.Agents {
		a, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("agent %q: %w", id, domain.ErrNotFound)
		}
		if id == pluginsAgentID {
			return nil, fmt.Errorf("agent %q: %w", id, domain.ErrReadOnly)
		}
		out = append(out, a)
	}
	return out, nil
}

// pluginsAgentID is the read-only Claude plugin cache pseudo-agent.
const pluginsAgentID domain.AgentID = "claude"

// linkPaths returns where the skill dir goes for agent (primary) and an
// extra symlink location (secondary, may be empty). Globally each agent
// gets its primary global dir; in a project every agent shares
// .agents/skills and Claude Code additionally reads .claude/skills.
func (s *Service) linkPaths(agent domain.Agent, root, name string) (primary, secondary string) {
	if root == "" {
		return filepath.Join(agent.GlobalDirs[0], name), ""
	}
	primary = filepath.Join(root, filepath.FromSlash(sharedProjectDir), name)
	if agent.ID == ClaudeCodeID {
		secondary = filepath.Join(root, filepath.FromSlash(claudeProjectDir), name)
	}
	return primary, secondary
}

// placeSkill writes entry at path as a symlink into the vault (or a copy).
// An existing path is accepted only when it already is this entry.
func (s *Service) placeSkill(entry domain.VaultEntry, path string, copy bool) (LinkResult, error) {
	res := LinkResult{Path: path, Copied: copy}
	existing, err := s.isSameEntry(entry, path)
	if err != nil {
		return res, err
	}
	if existing {
		res.Existing = true
		return res, nil
	}
	if _, err := os.Lstat(path); err == nil {
		return res, fmt.Errorf("%s %w and is not vault entry %q; remove it or disable it first", path, domain.ErrExists, entry.Name)
	}
	if copy {
		if err := skillfs.CopyTree(entry.Path, path); err != nil {
			return res, err
		}
		return res, nil
	}
	if err := skillfs.Symlink(entry.Path, path, false); err != nil {
		return res, err
	}
	return res, nil
}

// placeLink writes a relative symlink at path pointing to target (the
// primary project install) for agents that read a second directory.
func (s *Service) placeLink(entry domain.VaultEntry, target, path string) (LinkResult, error) {
	res := LinkResult{Path: path}
	existing, err := s.isSameEntry(entry, path)
	if err != nil {
		return res, err
	}
	if existing {
		res.Existing = true
		return res, nil
	}
	if _, err := os.Lstat(path); err == nil {
		return res, fmt.Errorf("%s %w and is not vault entry %q; remove it or disable it first", path, domain.ErrExists, entry.Name)
	}
	if err := skillfs.Symlink(target, path, true); err != nil {
		return res, err
	}
	return res, nil
}

// isSameEntry reports whether path already holds entry: a symlink that
// resolves into the entry's vault dir, or a copy with the same hash. A
// missing path is not an error.
func (s *Service) isSameEntry(entry domain.VaultEntry, path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			// Dangling link: not ours to keep.
			return false, fmt.Errorf("%s %w (dangling symlink)", path, domain.ErrExists)
		}
		vaultResolved, err := filepath.EvalSymlinks(entry.Path)
		if err != nil {
			return false, err
		}
		return resolved == vaultResolved, nil
	}
	if !info.IsDir() {
		return false, nil
	}
	hash, err := skillfs.HashDir(path)
	if err != nil {
		return false, err
	}
	return hash == entry.ContentHash, nil
}

// writeCommandStub writes .claude/commands/<name>.md next to the Claude
// Code skills dir (global or project). An existing stub is left alone.
func (s *Service) writeCommandStub(agent domain.Agent, root string, entry domain.VaultEntry, skillPath string) error {
	var dir string
	if root == "" {
		dir = filepath.Join(filepath.Dir(agent.GlobalDirs[0]), "commands")
	} else {
		dir = filepath.Join(root, ".claude", "commands")
	}
	p := filepath.Join(dir, entry.Name+".md")
	if _, err := os.Lstat(p); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	desc := ""
	if info, err := s.inspector.Inspect(entry.Path); err == nil {
		desc = info.Description
	}
	return os.WriteFile(p, convert.CommandStub(entry.Name, desc, skillPath), 0o644)
}

// UninstallFromVault removes the links of vault entry name for one agent
// (global, or in the given project). It returns the removed paths.
func (s *Service) UninstallFromVault(ctx context.Context, name string, agent domain.AgentID, project string) ([]string, error) {
	if _, err := s.vault.GetVault(ctx, name); err != nil {
		return nil, fmt.Errorf("vault entry %q: %w", name, err)
	}
	scope := domain.GlobalScope
	root := ""
	if project != "" {
		var err error
		root, err = cleanRoot(project)
		if err != nil {
			return nil, err
		}
		scope = domain.ProjectScope(root)
	}
	links, err := s.skills.List(ctx, domain.SkillFilter{Agent: agent, Scope: scope})
	if err != nil {
		return nil, err
	}
	removed := []string{}
	for _, l := range links {
		if l.VaultRef != name {
			continue
		}
		p := l.Path
		if l.State == domain.SkillDisabled && l.QuarantinePath != "" {
			p = l.QuarantinePath
			_ = os.Remove(sidecarPath(p))
		}
		if err := skillfs.RemoveEntry(p); err != nil {
			return removed, err
		}
		removed = append(removed, p)
	}
	if len(removed) == 0 {
		return removed, fmt.Errorf("vault entry %q is not installed for %s in %s: %w", name, agent, scope, domain.ErrNotFound)
	}
	projects := map[string]bool{}
	if root != "" {
		projects[root] = true
	}
	if err := s.rescan(ctx, projects); err != nil {
		return removed, err
	}
	return removed, nil
}
