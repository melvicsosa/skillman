package app

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/melvicsosa/skillman/internal/domain"
)

// Issue kinds reported by Doctor.
const (
	IssueInvalidFrontmatter = "invalid-frontmatter"
	IssueDanglingSymlink    = "dangling-symlink"
	IssueDrift              = "drift"
	IssueQuarantineConflict = "quarantine-conflict"
	IssueScanError          = "scan-error"
	IssueVaultBrokenLink    = "vault-broken-link"
	IssueVaultMissing       = "vault-missing"
	IssueRejectedConversion = "rejected-conversion"
)

// Issue is one problem found by Doctor. SkillID is set when the issue maps
// to exactly one skill row; drift lists every involved skill in Details.
type Issue struct {
	Kind    string   `json:"kind"`
	Agent   string   `json:"agent,omitempty"`
	Scope   string   `json:"scope,omitempty"`
	SkillID string   `json:"skillId,omitempty"`
	Name    string   `json:"name,omitempty"`
	Path    string   `json:"path,omitempty"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
	// SkillIDs lists every skill involved in a multi-skill issue (drift).
	SkillIDs []string `json:"skillIds,omitempty"`
}

// DoctorReport groups issues by kind.
type DoctorReport struct {
	Issues []Issue        `json:"issues"`
	Counts map[string]int `json:"counts"`
}

type inspected struct {
	agent domain.AgentID
	scope domain.Scope
	id    string
	name  string
	path  string
	hash  string
}

// Doctor re-reads the filesystem for every enabled agent and registered
// project and reports spec violations, dangling symlinks, drift between
// copies of the same skill, and quarantine entries whose original path is
// occupied again.
func (s *Service) Doctor(ctx context.Context) (DoctorReport, error) {
	report := DoctorReport{Issues: []Issue{}, Counts: map[string]int{}}
	agents, err := s.enabledAgents(ctx)
	if err != nil {
		return report, err
	}
	targets, err := s.scanTargets(ctx, agents, ScanOptions{})
	if err != nil {
		return report, err
	}
	vaultIdx := newVaultIndex(s.VaultRoot(), nil)
	var all []inspected
	for _, t := range targets {
		found, err := s.agents.Discover(t.agent, t.scope, t.root)
		if err != nil {
			report.Issues = append(report.Issues, Issue{Kind: IssueScanError, Agent: string(t.agent.ID), Scope: string(t.scope), Message: err.Error()})
			continue
		}
		for _, d := range found {
			id := domain.SkillID(t.agent.ID, t.scope, d.Path)
			base := Issue{Agent: string(t.agent.ID), Scope: string(t.scope), SkillID: id, Name: d.Name, Path: d.Path}
			if d.Dangling {
				base.Kind = IssueDanglingSymlink
				base.Message = fmt.Sprintf("symlink target %s does not exist", d.LinkTarget)
				if name := vaultIdx.nameForTarget(d.LinkTarget); name != "" {
					base.Kind = IssueVaultBrokenLink
					base.Message = fmt.Sprintf("symlink points at vault entry %q which no longer exists (%s)", name, d.LinkTarget)
				}
				report.Issues = append(report.Issues, base)
				continue
			}
			info, err := s.inspector.Inspect(d.Path)
			if err != nil {
				base.Kind = IssueScanError
				base.Message = err.Error()
				report.Issues = append(report.Issues, base)
				continue
			}
			if len(info.SpecIssues) > 0 {
				base.Kind = IssueInvalidFrontmatter
				base.Message = strings.Join(info.SpecIssues, "; ")
				base.Details = info.SpecIssues
				report.Issues = append(report.Issues, base)
			}
			all = append(all, inspected{agent: t.agent.ID, scope: t.scope, id: id, name: d.Name, path: d.Path, hash: info.ContentHash})
		}
	}
	report.Issues = append(report.Issues, driftIssues(all)...)

	disabled, err := s.skills.List(ctx, domain.SkillFilter{State: domain.SkillDisabled})
	if err != nil {
		return report, err
	}
	for _, sk := range disabled {
		if _, err := os.Lstat(sk.Path); err == nil {
			report.Issues = append(report.Issues, Issue{
				Kind: IssueQuarantineConflict, Agent: string(sk.AgentID), Scope: string(sk.Scope), SkillID: sk.ID,
				Name: sk.Name, Path: sk.Path,
				Message: fmt.Sprintf("disabled skill is quarantined at %s but %s exists again; enabling will fail until it is removed", sk.QuarantinePath, sk.Path),
			})
		}
	}
	if err := s.vaultIssues(ctx, &report); err != nil {
		return report, err
	}
	for _, i := range report.Issues {
		report.Counts[i.Kind]++
	}
	return report, nil
}

// vaultIssues reports vault entries whose directory is gone, sidecars
// without a directory, and recorded conversion rejections.
func (s *Service) vaultIssues(ctx context.Context, report *DoctorReport) error {
	entries, err := s.vault.ListVault(ctx)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if _, err := os.Stat(e.Path); err != nil {
			report.Issues = append(report.Issues, Issue{Kind: IssueVaultMissing, Name: e.Name, Path: e.Path,
				Message: fmt.Sprintf("vault entry %q has no directory; run `skillman vault remove %s` or re-add %s", e.Name, e.Name, e.Source.Ref)})
		}
	}
	rejections, err := s.Rejections()
	if err != nil {
		return err
	}
	for _, r := range rejections {
		report.Issues = append(report.Issues, Issue{Kind: IssueRejectedConversion, Name: r.Ref, Path: r.Ref,
			Message: fmt.Sprintf("%s could not be converted (%s): %s", r.Ref, r.Shape, r.Reason)})
	}
	return nil
}

// driftIssues groups skills by (scope, name) and reports groups whose
// copies do not share one content hash.
func driftIssues(all []inspected) []Issue {
	type key struct {
		scope domain.Scope
		name  string
	}
	groups := map[key][]inspected{}
	var order []key
	for _, i := range all {
		k := key{i.scope, i.name}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], i)
	}
	sort.Slice(order, func(a, b int) bool {
		if order[a].scope != order[b].scope {
			return order[a].scope < order[b].scope
		}
		return order[a].name < order[b].name
	})
	var out []Issue
	for _, k := range order {
		members := groups[k]
		hashes := map[string]bool{}
		for _, m := range members {
			hashes[m.hash] = true
		}
		if len(hashes) < 2 {
			continue
		}
		issue := Issue{Kind: IssueDrift, Scope: string(k.scope), Name: k.name,
			Message: fmt.Sprintf("%d copies with %d different content hashes", len(members), len(hashes))}
		for _, m := range members {
			issue.Details = append(issue.Details, fmt.Sprintf("%s %s %s", m.agent, shortHash(m.hash), m.path))
			issue.SkillIDs = append(issue.SkillIDs, m.id)
		}
		out = append(out, issue)
	}
	return out
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
