package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/domain"
)

// RepairSourceVault selects the vault entry as the drift repair source.
const RepairSourceVault = "vault"

// DriftCopy is one copy of a drifting skill, as listed on a drift issue.
type DriftCopy struct {
	SkillID    string `json:"skillId"`
	Agent      string `json:"agent"`
	Scope      string `json:"scope"`
	Path       string `json:"path"`
	Hash       string `json:"hash"`
	IsSymlink  bool   `json:"isSymlink"`
	VaultRef   string `json:"vaultRef,omitempty"`
	ReadOnly   bool   `json:"readOnly"`
	Repairable bool   `json:"repairable"`
	Reason     string `json:"reason,omitempty"` // why it is not repairable
}

// RepairOptions selects the drift group and the copy that wins.
type RepairOptions struct {
	Name string
	// From is RepairSourceVault or the agent whose copy is the source.
	From string
	// To limits the destinations to these agents; empty means every copy.
	To   []domain.AgentID
	Root string // project root of the drift group; "" for global
}

// RepairSkip is a copy the repair left alone and why.
type RepairSkip struct {
	Path   string `json:"path"`
	Agent  string `json:"agent"`
	Reason string `json:"reason"`
}

// RepairResult reports what RepairDrift did.
type RepairResult struct {
	Name       string       `json:"name"`
	From       string       `json:"from"`
	SourcePath string       `json:"sourcePath"`
	SourceHash string       `json:"sourceHash"`
	Replaced   []string     `json:"replaced"`
	Skipped    []RepairSkip `json:"skipped"`
}

// RepairDrift replaces the contents of every other copy of skill name in
// the scope with the source copy. Destination directories keep their
// path; symlinks into the vault and read-only plugin copies are skipped.
func (s *Service) RepairDrift(ctx context.Context, opts RepairOptions) (RepairResult, error) {
	res := RepairResult{Name: opts.Name, From: opts.From, Replaced: []string{}, Skipped: []RepairSkip{}}
	if opts.Name == "" || opts.From == "" {
		return res, errors.New("repair needs a skill name and --from <agent|vault>")
	}
	scope, root, err := scopeFor(opts.Root)
	if err != nil {
		return res, err
	}
	copies, err := s.driftCopies(ctx, opts.Name, scope, root)
	if err != nil {
		return res, err
	}
	if len(copies) == 0 {
		return res, fmt.Errorf("skill %q in %s: %w", opts.Name, scope, domain.ErrNotFound)
	}
	var sourceResolved string
	if opts.From == RepairSourceVault {
		entry, err := s.vault.GetVault(ctx, opts.Name)
		if err != nil {
			return res, fmt.Errorf("vault entry %q: %w", opts.Name, err)
		}
		res.SourcePath, res.SourceHash = entry.Path, entry.ContentHash
	} else {
		found := false
		for _, c := range copies {
			if c.Agent == opts.From {
				res.SourcePath, res.SourceHash, found = c.Path, c.Hash, true
				break
			}
		}
		if !found {
			return res, fmt.Errorf("agent %q has no copy of %q in %s: %w", opts.From, opts.Name, scope, domain.ErrNotFound)
		}
	}
	if sourceResolved, err = filepath.EvalSymlinks(res.SourcePath); err != nil {
		return res, fmt.Errorf("source %s: %w", res.SourcePath, err)
	}
	only := map[domain.AgentID]bool{}
	for _, a := range opts.To {
		only[a] = true
	}
	seen := map[string]bool{}
	changed := false
	for _, c := range copies {
		if seen[c.Path] || (len(only) > 0 && !only[domain.AgentID(c.Agent)]) {
			continue
		}
		seen[c.Path] = true
		if resolved, err := filepath.EvalSymlinks(c.Path); err == nil && resolved == sourceResolved {
			if c.Path == res.SourcePath {
				continue
			}
			reason := "already points at the source"
			if !c.Repairable {
				reason = c.Reason
			}
			res.Skipped = append(res.Skipped, RepairSkip{Path: c.Path, Agent: c.Agent, Reason: reason})
			continue
		}
		if c.Hash == res.SourceHash {
			res.Skipped = append(res.Skipped, RepairSkip{Path: c.Path, Agent: c.Agent, Reason: "already identical"})
			continue
		}
		if !c.Repairable {
			res.Skipped = append(res.Skipped, RepairSkip{Path: c.Path, Agent: c.Agent, Reason: c.Reason})
			continue
		}
		if err := replaceContents(res.SourcePath, c.Path); err != nil {
			return res, fmt.Errorf("repair %s: %w", c.Path, err)
		}
		res.Replaced = append(res.Replaced, c.Path)
		changed = true
	}
	if changed {
		projects := map[string]bool{}
		if root != "" {
			projects[root] = true
		}
		if err := s.rescan(ctx, projects); err != nil {
			return res, err
		}
	}
	return res, nil
}

// driftCopies lists every on-disk copy of skill name in scope across the
// enabled agents, with the vault attribution doctor uses.
func (s *Service) driftCopies(ctx context.Context, name string, scope domain.Scope, root string) ([]DriftCopy, error) {
	agents, err := s.enabledAgents(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := s.vault.ListVault(ctx)
	if err != nil {
		return nil, err
	}
	idx := newVaultIndex(s.VaultRoot(), entries)
	var out []DriftCopy
	for _, a := range agents {
		found, err := s.agents.Discover(a, scope, root)
		if err != nil {
			return nil, err
		}
		for _, d := range found {
			if d.Name != name || d.Dangling {
				continue
			}
			info, err := s.inspector.Inspect(d.Path)
			if err != nil {
				continue
			}
			out = append(out, driftCopy(a.ID, scope, d, info.ContentHash, idx))
		}
	}
	return out, nil
}

// driftCopy classifies one copy: repairable unless it is read-only or a
// symlink (into the vault or elsewhere).
func driftCopy(agent domain.AgentID, scope domain.Scope, d domain.DiscoveredSkill, hash string, idx vaultIndex) DriftCopy {
	c := DriftCopy{
		SkillID: domain.SkillID(agent, scope, d.Path), Agent: string(agent), Scope: string(scope), Path: d.Path,
		Hash: hash, IsSymlink: d.IsSymlink, ReadOnly: d.ReadOnly, VaultRef: idx.refFor(d.IsSymlink, d.LinkTarget, hash),
	}
	switch {
	case d.ReadOnly:
		c.Reason = "read-only plugin copy"
	case d.IsSymlink && idx.nameForTarget(d.LinkTarget) != "":
		c.Reason = "symlink into the vault (update the vault entry instead)"
	case d.IsSymlink:
		c.Reason = "symlink to " + d.LinkTarget
	default:
		c.Repairable = true
	}
	return c
}

// replaceContents swaps the tree at dest with a copy of src, keeping the
// dest path. src may be a symlink; its target tree is copied.
func replaceContents(src, dest string) error {
	resolved, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}
	tmp := dest + ".sync-tmp"
	_ = os.RemoveAll(tmp)
	if err := skillfs.CopyTree(resolved, tmp); err != nil {
		return err
	}
	if err := skillfs.ReplaceDir(tmp, dest); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	return nil
}

func scopeFor(root string) (domain.Scope, string, error) {
	if root == "" {
		return domain.GlobalScope, "", nil
	}
	clean, err := cleanRoot(root)
	if err != nil {
		return "", "", err
	}
	return domain.ProjectScope(clean), clean, nil
}

// VaultAdopt copies an agent's global skill into the vault as a local
// entry (ref = the original path) so it can become a sync source. With
// link the agent's directory is replaced by a symlink into the vault.
func (s *Service) VaultAdopt(ctx context.Context, name string, from domain.AgentID, link bool) (domain.VaultEntry, error) {
	if strings.TrimSpace(name) == "" || from == "" {
		return domain.VaultEntry{}, errors.New("adopt needs a skill name and --from <agent>")
	}
	if _, err := s.vault.GetVault(ctx, name); err == nil {
		return domain.VaultEntry{}, fmt.Errorf("vault entry %q %w; remove it first", name, domain.ErrExists)
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.VaultEntry{}, err
	}
	agents, err := s.enabledAgents(ctx)
	if err != nil {
		return domain.VaultEntry{}, err
	}
	var agent *domain.Agent
	for i := range agents {
		if agents[i].ID == from {
			agent = &agents[i]
		}
	}
	if agent == nil {
		return domain.VaultEntry{}, fmt.Errorf("agent %q is unknown or disabled: %w", from, domain.ErrNotFound)
	}
	found, err := s.agents.Discover(*agent, domain.GlobalScope, "")
	if err != nil {
		return domain.VaultEntry{}, err
	}
	var src *domain.DiscoveredSkill
	for i := range found {
		if found[i].Name == name && !found[i].Dangling {
			src = &found[i]
			break
		}
	}
	if src == nil {
		return domain.VaultEntry{}, fmt.Errorf("agent %s has no skill %q: %w", from, name, domain.ErrNotFound)
	}
	if link && src.ReadOnly {
		return domain.VaultEntry{}, fmt.Errorf("%s: cannot replace a read-only plugin copy with a link: %w", src.Path, domain.ErrReadOnly)
	}
	staging, err := os.MkdirTemp("", "skillman-adopt-*")
	if err != nil {
		return domain.VaultEntry{}, err
	}
	defer os.RemoveAll(staging)
	report, err := s.converter.Convert(src.Path, staging, name)
	if err != nil {
		return domain.VaultEntry{}, err
	}
	if len(report.Skills) != 1 {
		return domain.VaultEntry{}, fmt.Errorf("%s: expected one skill, got %d", src.Path, len(report.Skills))
	}
	hash, err := skillfs.HashDir(src.Path)
	if err != nil {
		return domain.VaultEntry{}, err
	}
	source := domain.VaultSource{Type: domain.SourceLocal, Ref: src.Path}
	entry, err := s.storeInVault(ctx, report.Skills[0], source, hash, time.Time{})
	if err != nil {
		return entry, err
	}
	if link {
		if err := skillfs.RemoveEntry(src.Path); err != nil {
			return entry, err
		}
		if err := skillfs.Symlink(entry.Path, src.Path, false); err != nil {
			return entry, err
		}
	}
	if err := s.rescan(ctx, nil); err != nil {
		return entry, err
	}
	return entry, nil
}
