package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/melvicsosa/skillman/internal/domain"
)

// ScanOptions narrows a scan. A nil ProjectRoot scans everything.
type ScanOptions struct {
	ProjectRoot *string
}

// ScanSummary is what a scan reports back.
type ScanSummary struct {
	Found    int      `json:"found"`
	Added    int      `json:"added"`
	Removed  int      `json:"removed"`
	Disabled int      `json:"disabled"`
	Errors   []string `json:"errors"`
	// Sync is the auto-sync fan-out result, nil when no entry has it on.
	Sync *SyncReport `json:"sync,omitempty"`
}

// originSidecarSuffix names the file written next to each quarantined
// entry so a scan can restore the row after the database is wiped.
const originSidecarSuffix = ".origin.json"

// origin is the content of a quarantine sidecar.
type origin struct {
	Agent domain.AgentID `json:"agent"`
	Scope domain.Scope   `json:"scope"`
	Path  string         `json:"path"`
	Name  string         `json:"name"`
}

type scanTarget struct {
	agent     domain.Agent
	scope     domain.Scope
	root      string
	projectID *int64
}

// Scan rebuilds the skill cache from disk for every enabled agent and every
// registered project (or only the given project), then re-reads the
// quarantine so disabled skills keep their rows.
func (s *Service) Scan(ctx context.Context, opts ScanOptions) (ScanSummary, error) {
	sum := ScanSummary{Errors: []string{}}
	if err := s.ensureAgentRows(ctx); err != nil {
		return sum, err
	}
	scanID, err := s.scans.Start(ctx)
	if err != nil {
		return sum, err
	}
	agents, err := s.enabledAgents(ctx)
	if err != nil {
		return sum, err
	}
	targets, err := s.scanTargets(ctx, agents, opts)
	if err != nil {
		return sum, err
	}
	entries, err := s.rebuildVault(ctx, &sum)
	if err != nil {
		return sum, err
	}
	if opts.ProjectRoot == nil {
		s.autoSync(ctx, entries, &sum)
	}
	idx := newVaultIndex(s.VaultRoot(), entries)
	for _, t := range targets {
		if err := s.scanTarget(ctx, t, idx, &sum); err != nil {
			sum.Errors = append(sum.Errors, fmt.Sprintf("%s %s: %v", t.agent.ID, t.scope, err))
		}
	}
	if err := s.scanQuarantine(ctx, idx, &sum); err != nil {
		sum.Errors = append(sum.Errors, "quarantine: "+err.Error())
	}
	if err := s.scans.Finish(ctx, scanID, sum.Found, sum.Errors); err != nil {
		return sum, err
	}
	return sum, nil
}

func (s *Service) scanTargets(ctx context.Context, agents []domain.Agent, opts ScanOptions) ([]scanTarget, error) {
	var targets []scanTarget
	if opts.ProjectRoot != nil {
		root, err := cleanRoot(*opts.ProjectRoot)
		if err != nil {
			return nil, err
		}
		p, err := s.projects.GetByRoot(ctx, root)
		if err != nil {
			return nil, fmt.Errorf("project %s: %w", root, err)
		}
		for _, a := range agents {
			targets = append(targets, scanTarget{agent: a, scope: domain.ProjectScope(root), root: root, projectID: &p.ID})
		}
		return targets, nil
	}
	for _, a := range agents {
		targets = append(targets, scanTarget{agent: a, scope: domain.GlobalScope})
	}
	projects, err := s.projects.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		id := p.ID
		for _, a := range agents {
			targets = append(targets, scanTarget{agent: a, scope: domain.ProjectScope(p.Root), root: p.Root, projectID: &id})
		}
	}
	return targets, nil
}

func (s *Service) scanTarget(ctx context.Context, t scanTarget, idx vaultIndex, sum *ScanSummary) error {
	existing, err := s.skills.List(ctx, domain.SkillFilter{Agent: t.agent.ID, Scope: t.scope})
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, sk := range existing {
		known[sk.ID] = true
	}
	found, err := s.agents.Discover(t.agent, t.scope, t.root)
	if err != nil {
		return err
	}
	now := time.Now()
	var (
		batch []domain.Skill
		seen  []string
	)
	for _, d := range found {
		if d.Dangling {
			sum.Errors = append(sum.Errors, fmt.Sprintf("%s: dangling symlink %s -> %s", t.agent.ID, d.Path, d.LinkTarget))
			continue
		}
		info, err := s.inspector.Inspect(d.Path)
		if err != nil {
			sum.Errors = append(sum.Errors, fmt.Sprintf("%s: %v", t.agent.ID, err))
			continue
		}
		sk := domain.Skill{
			ID: domain.SkillID(t.agent.ID, t.scope, d.Path), AgentID: t.agent.ID, Scope: t.scope,
			ProjectID: t.projectID, Path: d.Path, Name: d.Name, Description: info.Description,
			Version: info.Version, ContentHash: info.ContentHash, IsSymlink: d.IsSymlink,
			LinkTarget: d.LinkTarget, State: domain.SkillEnabled, ReadOnly: d.ReadOnly, LastSeenAt: now,
			VaultRef: idx.refFor(d.IsSymlink, d.LinkTarget, info.ContentHash),
		}
		batch = append(batch, sk)
		seen = append(seen, sk.ID)
		if !known[sk.ID] {
			sum.Added++
		}
	}
	if err := s.skills.UpsertMany(ctx, batch); err != nil {
		return err
	}
	removed, err := s.skills.MarkMissing(ctx, t.agent.ID, t.scope, seen)
	if err != nil {
		return err
	}
	sum.Found += len(batch)
	sum.Removed += removed
	return nil
}

// scanQuarantine walks <dataDir>/disabled/** and upserts one disabled row
// per sidecar whose entry still exists.
func (s *Service) scanQuarantine(ctx context.Context, idx vaultIndex, sum *ScanSummary) error {
	root := s.QuarantineRoot()
	var batch []domain.Skill
	now := time.Now()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), originSidecarSuffix) {
			return nil
		}
		o, err := readOrigin(p)
		if err != nil {
			sum.Errors = append(sum.Errors, fmt.Sprintf("quarantine: %v", err))
			return nil
		}
		entry := strings.TrimSuffix(p, originSidecarSuffix)
		if _, err := os.Lstat(entry); err != nil {
			sum.Errors = append(sum.Errors, fmt.Sprintf("quarantine: sidecar %s has no entry", p))
			return nil
		}
		sk := domain.Skill{
			ID: domain.SkillID(o.Agent, o.Scope, o.Path), AgentID: o.Agent, Scope: o.Scope, Path: o.Path,
			Name: o.Name, State: domain.SkillDisabled, QuarantinePath: entry, LastSeenAt: now,
		}
		if root := o.Scope.ProjectRoot(); root != "" {
			if proj, err := s.projects.GetByRoot(ctx, root); err == nil {
				sk.ProjectID = &proj.ID
			}
		}
		if info, err := os.Lstat(entry); err == nil && info.Mode()&fs.ModeSymlink != 0 {
			sk.IsSymlink = true
			if target, err := filepath.EvalSymlinks(entry); err == nil {
				sk.LinkTarget = target
			}
		}
		if info, err := s.inspector.Inspect(entry); err == nil {
			sk.Description, sk.Version, sk.ContentHash = info.Description, info.Version, info.ContentHash
		}
		sk.VaultRef = idx.refFor(sk.IsSymlink, sk.LinkTarget, sk.ContentHash)
		batch = append(batch, sk)
		return nil
	})
	if err != nil {
		return err
	}
	if err := s.skills.UpsertMany(ctx, batch); err != nil {
		return err
	}
	sum.Disabled = len(batch)
	return nil
}

func sidecarPath(entry string) string { return entry + originSidecarSuffix }

func writeOrigin(entry string, o origin) error {
	b, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sidecarPath(entry), b, 0o644)
}

func readOrigin(p string) (origin, error) {
	var o origin
	b, err := os.ReadFile(p)
	if err != nil {
		return o, err
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return o, fmt.Errorf("%s: %w", p, err)
	}
	if o.Agent == "" || o.Scope == "" || o.Path == "" {
		return o, fmt.Errorf("%s: incomplete origin", p)
	}
	if o.Name == "" {
		o.Name = filepath.Base(o.Path)
	}
	return o, nil
}

// cleanRoot returns the absolute, cleaned form of a project root.
func cleanRoot(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}
