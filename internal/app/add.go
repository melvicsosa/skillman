package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	skillfs "github.com/melvicsosa/skillman/internal/adapters/fs"
	"github.com/melvicsosa/skillman/internal/domain"
)

// AddResult reports what Add stored and linked.
type AddResult struct {
	Shape    domain.SourceShape  `json:"shape"`
	Entries  []domain.VaultEntry `json:"entries"`
	Links    []LinkResult        `json:"links"`
	Warnings []string            `json:"warnings"`
}

// Add resolves ref, fetches it, converts it to spec skills, stores them in
// the vault with provenance and installs them into the requested agents
// (PLAN.md section 5). Re-adding the same ref replaces the entry; a
// different ref for an existing name fails with ErrExists.
func (s *Service) Add(ctx context.Context, ref string, opts InstallOptions) (AddResult, error) {
	res := AddResult{Entries: []domain.VaultEntry{}, Links: []LinkResult{}, Warnings: []string{}}
	reg, err := s.ResolveRef(ref)
	if err != nil {
		return res, err
	}
	fetched, err := reg.Fetch(ctx, ref)
	if err != nil {
		return res, err
	}
	defer fetched.Cleanup()
	sourceHash, err := skillfs.HashDir(fetched.Dir)
	if err != nil {
		return res, err
	}
	staging, err := os.MkdirTemp("", "skillman-convert-*")
	if err != nil {
		return res, err
	}
	defer os.RemoveAll(staging)
	report, err := s.converter.Convert(fetched.Dir, staging, fetched.Name)
	if err != nil {
		if errors.Is(err, domain.ErrRejected) {
			s.recordRejection(ref, report.Shape, err.Error())
		}
		return res, err
	}
	res.Shape = report.Shape
	for _, sk := range report.Skills {
		res.Warnings = append(res.Warnings, prefixed(sk.Name, sk.Warnings)...)
		installedAt := time.Time{}
		if existing, err := s.vault.GetVault(ctx, sk.Name); err == nil {
			if existing.Source.Ref != fetched.Source.Ref {
				return res, fmt.Errorf("vault entry %q %w (from %s); run `skillman vault remove %s` first", sk.Name, domain.ErrExists, existing.Source.Ref, sk.Name)
			}
			installedAt = existing.InstalledAt
		} else if !errors.Is(err, domain.ErrNotFound) {
			return res, err
		}
		entry, err := s.storeInVault(ctx, sk, fetched.Source, sourceHash, installedAt)
		if err != nil {
			return res, err
		}
		res.Entries = append(res.Entries, entry)
	}
	if len(opts.Agents) == 0 && !opts.All {
		if _, err := s.Scan(ctx, ScanOptions{}); err != nil {
			return res, err
		}
		return res, nil
	}
	for _, e := range res.Entries {
		links, err := s.InstallFromVault(ctx, e.Name, opts)
		if err != nil {
			return res, err
		}
		res.Links = append(res.Links, links...)
	}
	return res, nil
}

func prefixed(name string, warnings []string) []string {
	out := make([]string, 0, len(warnings))
	for _, w := range warnings {
		out = append(out, name+": "+w)
	}
	return out
}

// Rejection is a conversion doctor reports (PLAN.md section 4 step 4).
type Rejection struct {
	Ref    string             `json:"ref"`
	Shape  domain.SourceShape `json:"shape"`
	Reason string             `json:"reason"`
	At     time.Time          `json:"at"`
}

const (
	rejectionsFile = "rejected.json"
	maxRejections  = 50
)

func (s *Service) rejectionsPath() string { return filepath.Join(s.dataDir, rejectionsFile) }

// recordRejection appends to <dataDir>/rejected.json, replacing an older
// entry for the same ref and keeping the newest maxRejections.
func (s *Service) recordRejection(ref string, shape domain.SourceShape, reason string) {
	list, _ := s.Rejections()
	kept := list[:0]
	for _, r := range list {
		if r.Ref != ref {
			kept = append(kept, r)
		}
	}
	kept = append(kept, Rejection{Ref: ref, Shape: shape, Reason: reason, At: time.Now()})
	if len(kept) > maxRejections {
		kept = kept[len(kept)-maxRejections:]
	}
	b, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(s.dataDir, 0o755)
	_ = os.WriteFile(s.rejectionsPath(), b, 0o644)
}

// Rejections lists the recorded conversion rejections, oldest first.
func (s *Service) Rejections() ([]Rejection, error) {
	b, err := os.ReadFile(s.rejectionsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []Rejection{}, nil
		}
		return nil, err
	}
	var list []Rejection
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, fmt.Errorf("%s: %w", s.rejectionsPath(), err)
	}
	return list, nil
}

// ClearRejections forgets every recorded rejection.
func (s *Service) ClearRejections() error {
	err := os.Remove(s.rejectionsPath())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
