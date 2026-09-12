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

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/domain"
)

// VaultDirName is the subdirectory of the data dir holding downloaded skills.
const VaultDirName = "vault"

// vaultSidecarSuffix names the provenance file written next to each vault
// entry: <vault>/<name>.vault.json. The sidecar is the source of truth; the
// database row is a cache rebuilt from it on every scan.
const vaultSidecarSuffix = ".vault.json"

// VaultRoot is <dataDir>/vault.
func (s *Service) VaultRoot() string { return filepath.Join(s.dataDir, VaultDirName) }

func (s *Service) vaultPath(name string) string { return filepath.Join(s.VaultRoot(), name) }

func (s *Service) vaultSidecar(name string) string { return s.vaultPath(name) + vaultSidecarSuffix }

// VaultStatus is a vault entry with the skills currently linked to it.
type VaultStatus struct {
	domain.VaultEntry
	Links []domain.Skill `json:"links"`
}

// VaultList returns every entry with its links, ordered by name.
func (s *Service) VaultList(ctx context.Context) ([]VaultStatus, error) {
	entries, err := s.vault.ListVault(ctx)
	if err != nil {
		return nil, err
	}
	links, err := s.linksByVault(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]VaultStatus, 0, len(entries))
	for _, e := range entries {
		l := links[e.Name]
		if l == nil {
			l = []domain.Skill{}
		}
		out = append(out, VaultStatus{VaultEntry: e, Links: l})
	}
	return out, nil
}

// VaultGet returns one entry with its links.
func (s *Service) VaultGet(ctx context.Context, name string) (VaultStatus, error) {
	e, err := s.vault.GetVault(ctx, name)
	if err != nil {
		return VaultStatus{}, fmt.Errorf("vault entry %q: %w", name, err)
	}
	links, err := s.vaultLinks(ctx, name)
	if err != nil {
		return VaultStatus{}, err
	}
	return VaultStatus{VaultEntry: e, Links: links}, nil
}

// vaultLinks lists the cached skills installed from the vault entry name.
func (s *Service) vaultLinks(ctx context.Context, name string) ([]domain.Skill, error) {
	all, err := s.linksByVault(ctx)
	if err != nil {
		return nil, err
	}
	l := all[name]
	if l == nil {
		l = []domain.Skill{}
	}
	return l, nil
}

func (s *Service) linksByVault(ctx context.Context) (map[string][]domain.Skill, error) {
	skills, err := s.skills.List(ctx, domain.SkillFilter{})
	if err != nil {
		return nil, err
	}
	out := map[string][]domain.Skill{}
	for _, sk := range skills {
		if sk.VaultRef != "" {
			out[sk.VaultRef] = append(out[sk.VaultRef], sk)
		}
	}
	return out, nil
}

// storeInVault moves a converted skill directory into vault/<name>,
// replacing any existing entry, then writes the sidecar and the cache row.
func (s *Service) storeInVault(ctx context.Context, converted domain.ConvertedSkill, source domain.VaultSource, sourceHash string, installedAt time.Time) (domain.VaultEntry, error) {
	if err := os.MkdirAll(s.VaultRoot(), 0o755); err != nil {
		return domain.VaultEntry{}, err
	}
	dest := s.vaultPath(converted.Name)
	if err := skillfs.ReplaceDir(converted.Path, dest); err != nil {
		return domain.VaultEntry{}, fmt.Errorf("store %s in vault: %w", converted.Name, err)
	}
	info, err := s.inspector.Inspect(dest)
	if err != nil {
		return domain.VaultEntry{}, err
	}
	now := time.Now()
	if installedAt.IsZero() {
		installedAt = now
	}
	entry := domain.VaultEntry{
		Name: converted.Name, Path: dest, ContentHash: info.ContentHash, Source: source, SourceHash: sourceHash,
		InstalledAt: installedAt, UpdatedAt: now, ConvertedFrom: converted.ConvertedFrom,
		Spec: domain.SpecReport{Valid: len(info.SpecIssues) == 0, Issues: nonNil(info.SpecIssues)},
	}
	if err := s.writeVaultEntry(ctx, entry); err != nil {
		return entry, err
	}
	return entry, nil
}

func (s *Service) writeVaultEntry(ctx context.Context, entry domain.VaultEntry) error {
	b, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.vaultSidecar(entry.Name), b, 0o644); err != nil {
		return fmt.Errorf("write vault sidecar: %w", err)
	}
	return s.vault.UpsertVault(ctx, entry)
}

func readVaultSidecar(p string) (domain.VaultEntry, error) {
	var e domain.VaultEntry
	b, err := os.ReadFile(p)
	if err != nil {
		return e, err
	}
	if err := json.Unmarshal(b, &e); err != nil {
		return e, fmt.Errorf("%s: %w", p, err)
	}
	if e.Name == "" {
		e.Name = strings.TrimSuffix(filepath.Base(p), vaultSidecarSuffix)
	}
	e.Path = strings.TrimSuffix(p, vaultSidecarSuffix)
	e.Spec.Issues = nonNil(e.Spec.Issues)
	return e, nil
}

// rebuildVault re-reads every <vault>/<name>.vault.json sidecar into the
// cache and drops rows whose sidecar or directory is gone. It returns the
// entries so the scan can attribute skills to them.
func (s *Service) rebuildVault(ctx context.Context, sum *ScanSummary) ([]domain.VaultEntry, error) {
	cached, err := s.vault.ListVault(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var entries []domain.VaultEntry
	items, err := os.ReadDir(s.VaultRoot())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, it := range items {
		if it.IsDir() || !strings.HasSuffix(it.Name(), vaultSidecarSuffix) {
			continue
		}
		p := filepath.Join(s.VaultRoot(), it.Name())
		e, err := readVaultSidecar(p)
		if err != nil {
			sum.Errors = append(sum.Errors, "vault: "+err.Error())
			continue
		}
		if _, err := os.Stat(filepath.Join(e.Path, skillfs.SkillFile)); err != nil {
			sum.Errors = append(sum.Errors, fmt.Sprintf("vault: %s has a sidecar but no skill directory", e.Name))
			continue
		}
		if info, err := s.inspector.Inspect(e.Path); err == nil {
			e.ContentHash = info.ContentHash
			e.Spec = domain.SpecReport{Valid: len(info.SpecIssues) == 0, Issues: nonNil(info.SpecIssues)}
		}
		if err := s.vault.UpsertVault(ctx, e); err != nil {
			return nil, err
		}
		seen[e.Name] = true
		entries = append(entries, e)
	}
	for _, c := range cached {
		if !seen[c.Name] {
			if err := s.vault.DeleteVault(ctx, c.Name); err != nil {
				return nil, err
			}
		}
	}
	return entries, nil
}

// vaultIndex resolves a discovered skill to the vault entry it comes from:
// a symlink into the vault, or a copy whose hash equals an entry's hash.
type vaultIndex struct {
	root   string
	byHash map[string]string
}

func newVaultIndex(root string, entries []domain.VaultEntry) vaultIndex {
	idx := vaultIndex{root: root, byHash: map[string]string{}}
	for _, e := range entries {
		if e.ContentHash != "" {
			idx.byHash[e.ContentHash] = e.Name
		}
	}
	return idx
}

// refFor returns the vault entry name for a link target or content hash.
func (idx vaultIndex) refFor(isSymlink bool, linkTarget, hash string) string {
	if isSymlink && linkTarget != "" {
		if name := idx.nameForTarget(linkTarget); name != "" {
			return name
		}
	}
	return idx.byHash[hash]
}

func (idx vaultIndex) nameForTarget(target string) string {
	if idx.root == "" || !skillfs.IsWithin(target, idx.root) {
		return ""
	}
	rel, err := filepath.Rel(idx.root, target)
	if err != nil || rel == "." {
		return ""
	}
	return strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
}

// VaultRemoveResult reports what VaultRemove did.
type VaultRemoveResult struct {
	Name     string   `json:"name"`
	Unlinked []string `json:"unlinked"`
}

// VaultRemove deletes a vault entry. It refuses with ErrLinked while skills
// still point at it unless force is set, in which case the links (symlinks
// or copies) are removed first.
func (s *Service) VaultRemove(ctx context.Context, name string, force bool) (VaultRemoveResult, error) {
	res := VaultRemoveResult{Name: name, Unlinked: []string{}}
	entry, err := s.vault.GetVault(ctx, name)
	if err != nil {
		return res, fmt.Errorf("vault entry %q: %w", name, err)
	}
	links, err := s.vaultLinks(ctx, name)
	if err != nil {
		return res, err
	}
	if len(links) > 0 && !force {
		paths := make([]string, 0, len(links))
		for _, l := range links {
			paths = append(paths, fmt.Sprintf("%s (%s)", l.Path, l.AgentID))
		}
		return res, fmt.Errorf("vault entry %q is %w: %s; use --force to unlink", name, domain.ErrLinked, strings.Join(paths, ", "))
	}
	projects := map[string]bool{}
	seen := map[string]bool{} // several agents share ~/.agents/skills
	for _, l := range links {
		p := l.Path
		if l.State == domain.SkillDisabled && l.QuarantinePath != "" {
			p = l.QuarantinePath
			_ = os.Remove(sidecarPath(p))
		}
		if root := l.Scope.ProjectRoot(); root != "" {
			projects[root] = true
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		if err := skillfs.RemoveEntry(p); err != nil {
			return res, fmt.Errorf("unlink %s: %w", p, err)
		}
		res.Unlinked = append(res.Unlinked, p)
	}
	if err := skillfs.RemoveEntry(entry.Path); err != nil {
		return res, err
	}
	if err := os.Remove(s.vaultSidecar(name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return res, err
	}
	if err := s.vault.DeleteVault(ctx, name); err != nil {
		return res, err
	}
	if len(links) > 0 {
		if err := s.rescan(ctx, projects); err != nil {
			return res, err
		}
	}
	return res, nil
}

// VaultUpdateResult reports what VaultUpdate did.
type VaultUpdateResult struct {
	Name          string   `json:"name"`
	Updated       bool     `json:"updated"`
	OldSourceHash string   `json:"oldSourceHash"`
	NewSourceHash string   `json:"newSourceHash"`
	CopiesRefresh []string `json:"copiesRefreshed"`
	Warnings      []string `json:"warnings"`
}

// VaultUpdate re-fetches an entry from its recorded source and replaces the
// vault directory when the source content changed. Symlinks keep working
// unchanged; installed copies are refreshed.
func (s *Service) VaultUpdate(ctx context.Context, name string) (VaultUpdateResult, error) {
	res := VaultUpdateResult{Name: name, CopiesRefresh: []string{}, Warnings: []string{}}
	entry, err := s.vault.GetVault(ctx, name)
	if err != nil {
		return res, fmt.Errorf("vault entry %q: %w", name, err)
	}
	res.OldSourceHash = entry.SourceHash
	reg, err := s.ResolveRef(entry.Source.Ref)
	if err != nil {
		return res, fmt.Errorf("update %s: %w", name, err)
	}
	fetched, err := reg.Fetch(ctx, entry.Source.Ref)
	if err != nil {
		return res, err
	}
	defer fetched.Cleanup()
	sourceHash, err := skillfs.HashDir(fetched.Dir)
	if err != nil {
		return res, err
	}
	res.NewSourceHash = sourceHash
	if sourceHash == entry.SourceHash {
		return res, nil
	}
	staging, err := os.MkdirTemp("", "skillman-convert-*")
	if err != nil {
		return res, err
	}
	defer os.RemoveAll(staging)
	report, err := s.converter.Convert(fetched.Dir, staging, fetched.Name)
	if err != nil {
		return res, err
	}
	var match *domain.ConvertedSkill
	for i := range report.Skills {
		if report.Skills[i].Name == name {
			match = &report.Skills[i]
		}
	}
	if match == nil {
		return res, fmt.Errorf("update %s: source no longer produces a skill named %q", name, name)
	}
	res.Warnings = append(res.Warnings, match.Warnings...)
	source := fetched.Source
	if source.Type == "" {
		source = entry.Source
	}
	if entry.Source.Type == domain.SourceLock {
		source.Type = domain.SourceLock
	}
	if _, err := s.storeInVault(ctx, *match, source, sourceHash, entry.InstalledAt); err != nil {
		return res, err
	}
	res.Updated = true
	links, err := s.vaultLinks(ctx, name)
	if err != nil {
		return res, err
	}
	projects := map[string]bool{}
	seen := map[string]bool{}
	for _, l := range links {
		if root := l.Scope.ProjectRoot(); root != "" {
			projects[root] = true
		}
		if l.IsSymlink || l.State != domain.SkillEnabled || seen[l.Path] {
			continue
		}
		seen[l.Path] = true
		if err := skillfs.RemoveEntry(l.Path); err != nil {
			return res, err
		}
		if err := skillfs.CopyTree(entry.Path, l.Path); err != nil {
			return res, err
		}
		res.CopiesRefresh = append(res.CopiesRefresh, l.Path)
	}
	if err := s.rescan(ctx, projects); err != nil {
		return res, err
	}
	return res, nil
}

// rescan runs a global scan (which also covers every registered project);
// unregistered project roots in projects are registered first.
func (s *Service) rescan(ctx context.Context, projects map[string]bool) error {
	for root := range projects {
		if _, err := s.projects.GetByRoot(ctx, root); errors.Is(err, domain.ErrNotFound) {
			if _, _, err := s.RegisterProject(ctx, root, RegisterProjectOptions{}); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	_, err := s.Scan(ctx, ScanOptions{})
	return err
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
