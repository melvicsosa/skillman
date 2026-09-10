package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/domain"
)

// Sync link statuses.
const (
	SyncLinked   = "linked"
	SyncCopied   = "copied"
	SyncAlready  = "already"
	SyncConflict = "conflict"
)

// SyncOptions selects which entries fan out and where.
type SyncOptions struct {
	Names  []string
	All    bool
	Root   string // project root; "" syncs the global scope
	DryRun bool
}

// SyncLink is the outcome for one (agent, path) of a fan-out.
type SyncLink struct {
	Agent   string `json:"agent"`
	Scope   string `json:"scope"`
	Path    string `json:"path"`
	Status  string `json:"status"` // one of the Sync* constants
	Message string `json:"message,omitempty"`
}

// SyncEntry is the fan-out result of one vault entry.
type SyncEntry struct {
	Name  string     `json:"name"`
	Links []SyncLink `json:"links"`
}

// SyncReport is what Sync (and the auto-sync step of Scan) reports.
type SyncReport struct {
	DryRun    bool        `json:"dryRun"`
	Entries   []SyncEntry `json:"entries"`
	Linked    int         `json:"linked"`
	Already   int         `json:"already"`
	Conflicts int         `json:"conflicts"`
}

func (r *SyncReport) add(e SyncEntry) {
	r.Entries = append(r.Entries, e)
	for _, l := range e.Links {
		switch l.Status {
		case SyncLinked, SyncCopied:
			r.Linked++
		case SyncAlready:
			r.Already++
		case SyncConflict:
			r.Conflicts++
		}
	}
}

// Sync makes sure the selected vault entries are installed in every enabled
// writable agent (global scope, or the project in opts.Root). An entry whose
// existing installs are copies is copied; otherwise it is symlinked. A
// foreign skill dir with the same name is reported as a conflict and left
// alone. Nothing is written in dry-run mode.
func (s *Service) Sync(ctx context.Context, opts SyncOptions) (SyncReport, error) {
	report := SyncReport{DryRun: opts.DryRun, Entries: []SyncEntry{}}
	if len(opts.Names) == 0 && !opts.All {
		return report, errors.New("nothing to sync: pass entry names or --all")
	}
	var entries []domain.VaultEntry
	if opts.All {
		all, err := s.vault.ListVault(ctx)
		if err != nil {
			return report, err
		}
		entries = all
	} else {
		for _, name := range opts.Names {
			e, err := s.vault.GetVault(ctx, name)
			if err != nil {
				return report, fmt.Errorf("vault entry %q: %w", name, err)
			}
			entries = append(entries, e)
		}
	}
	targets, err := s.installTargets(ctx, InstallOptions{All: true})
	if err != nil {
		return report, err
	}
	root := ""
	if opts.Root != "" {
		if root, err = cleanRoot(opts.Root); err != nil {
			return report, err
		}
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			return report, fmt.Errorf("project root %s is not a directory", root)
		}
	}
	changed, err := s.syncEntries(entries, targets, root, opts.DryRun, &report)
	if err != nil {
		return report, err
	}
	if changed {
		projects := map[string]bool{}
		if root != "" {
			projects[root] = true
		}
		if err := s.rescan(ctx, projects); err != nil {
			return report, err
		}
	}
	return report, nil
}

// syncEntries fans entries out to targets without rescanning. It reports
// whether anything was written.
func (s *Service) syncEntries(entries []domain.VaultEntry, targets []domain.Agent, root string, dryRun bool, report *SyncReport) (bool, error) {
	changed := false
	scope := string(domain.GlobalScope)
	if root != "" {
		scope = string(domain.ProjectScope(root))
	}
	for _, entry := range entries {
		if _, err := os.Stat(filepath.Join(entry.Path, skillfs.SkillFile)); err != nil {
			report.add(SyncEntry{Name: entry.Name, Links: []SyncLink{{Scope: scope, Path: entry.Path, Status: SyncConflict,
				Message: "vault entry has no skill directory"}}})
			continue
		}
		copyMode := s.entryUsesCopy(entry, targets, root)
		se := SyncEntry{Name: entry.Name, Links: []SyncLink{}}
		done := map[string]SyncLink{}
		for _, agent := range targets {
			primary, secondary := s.linkPaths(agent, root, entry.Name)
			for i, p := range []string{primary, secondary} {
				if p == "" {
					continue
				}
				l, seen := done[p]
				if !seen {
					var wrote bool
					var err error
					l, wrote, err = s.syncPath(entry, p, primary, i == 1, copyMode, dryRun)
					if err != nil {
						return changed, err
					}
					changed = changed || wrote
					done[p] = l
				}
				l.Agent, l.Scope = string(agent.ID), scope
				se.Links = append(se.Links, l)
			}
		}
		report.add(se)
	}
	return changed, nil
}

// syncPath places entry at path (a symlink to primary when secondary) and
// classifies the outcome. It never overwrites a foreign directory.
func (s *Service) syncPath(entry domain.VaultEntry, path, primary string, secondary, copyMode, dryRun bool) (SyncLink, bool, error) {
	l := SyncLink{Path: path, Status: SyncLinked}
	if copyMode && !secondary {
		l.Status = SyncCopied
	}
	same, err := s.isSameEntry(entry, path)
	if err != nil {
		if errors.Is(err, domain.ErrExists) {
			l.Status, l.Message = SyncConflict, "dangling symlink"
			return l, false, nil
		}
		return l, false, err
	}
	if same {
		l.Status = SyncAlready
		return l, false, nil
	}
	if _, err := os.Lstat(path); err == nil {
		l.Status, l.Message = SyncConflict, fmt.Sprintf("%s exists and is not vault entry %q", path, entry.Name)
		return l, false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return l, false, err
	}
	if dryRun {
		return l, false, nil
	}
	switch {
	case secondary:
		err = skillfs.Symlink(primary, path, true)
	case copyMode:
		err = skillfs.CopyTree(entry.Path, path)
	default:
		err = skillfs.Symlink(entry.Path, path, false)
	}
	if err != nil {
		return l, false, err
	}
	return l, true, nil
}

// entryUsesCopy reports whether entry is already installed as a copy (a
// real directory with the entry's hash) in one of the target locations,
// in which case the fan-out copies too instead of symlinking.
func (s *Service) entryUsesCopy(entry domain.VaultEntry, targets []domain.Agent, root string) bool {
	for _, agent := range targets {
		primary, _ := s.linkPaths(agent, root, entry.Name)
		info, err := os.Lstat(primary)
		if err != nil || info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		if hash, err := skillfs.HashDir(primary); err == nil && hash == entry.ContentHash {
			return true
		}
	}
	return false
}

// VaultSetAutoSync persists the auto-sync flag of one entry.
func (s *Service) VaultSetAutoSync(ctx context.Context, name string, on bool) (domain.VaultEntry, error) {
	entry, err := s.vault.GetVault(ctx, name)
	if err != nil {
		return entry, fmt.Errorf("vault entry %q: %w", name, err)
	}
	entry.AutoSync = on
	if err := s.writeVaultEntry(ctx, entry); err != nil {
		return entry, err
	}
	return entry, nil
}

// autoSync fans every auto-sync entry out globally. It runs inside Scan
// before agent dirs are read so the new links are picked up by the same
// scan; it never rescans itself.
func (s *Service) autoSync(ctx context.Context, entries []domain.VaultEntry, sum *ScanSummary) {
	var selected []domain.VaultEntry
	for _, e := range entries {
		if e.AutoSync {
			selected = append(selected, e)
		}
	}
	if len(selected) == 0 {
		return
	}
	targets, err := s.installTargets(ctx, InstallOptions{All: true})
	if err != nil {
		sum.Errors = append(sum.Errors, "auto-sync: "+err.Error())
		return
	}
	report := SyncReport{Entries: []SyncEntry{}}
	if _, err := s.syncEntries(selected, targets, "", false, &report); err != nil {
		sum.Errors = append(sum.Errors, "auto-sync: "+err.Error())
	}
	sum.Sync = &report
}
