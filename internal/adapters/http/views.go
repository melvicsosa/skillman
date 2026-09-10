package http

import (
	"time"

	"github.com/melvicsosa/skillman/internal/app"
	"github.com/melvicsosa/skillman/internal/domain"
)

// AgentView is the JSON shape of an agent, shared by the API and the CLI.
type AgentView struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Enabled     bool     `json:"enabled"`
	GlobalDirs  []string `json:"globalDirs"`
	ProjectDirs []string `json:"projectDirs"`
	Exists      bool     `json:"exists"`
	SkillCount  int      `json:"skillCount"`
	ReadOnly    bool     `json:"readOnly"`
}

// SkillView is the JSON shape of a skill, shared by the API and the CLI.
type SkillView struct {
	ID             string `json:"id"`
	Agent          string `json:"agent"`
	Scope          string `json:"scope"`
	ProjectRoot    string `json:"projectRoot,omitempty"`
	Path           string `json:"path"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Version        string `json:"version"`
	ContentHash    string `json:"contentHash"`
	IsSymlink      bool   `json:"isSymlink"`
	LinkTarget     string `json:"linkTarget,omitempty"`
	State          string `json:"state"`
	QuarantinePath string `json:"quarantinePath,omitempty"`
	ReadOnly       bool   `json:"readOnly"`
	VaultRef       string `json:"vaultRef,omitempty"`
	LastSeenAt     string `json:"lastSeenAt"`
}

// ProjectView is the JSON shape of a registered project.
type ProjectView struct {
	ID           int64  `json:"id"`
	Root         string `json:"root"`
	Name         string `json:"name"`
	RegisteredAt string `json:"registeredAt"`
}

// ToAgentViews converts agent statuses.
func ToAgentViews(list []app.AgentStatus) []AgentView {
	out := make([]AgentView, 0, len(list))
	for _, a := range list {
		out = append(out, ToAgentView(a))
	}
	return out
}

// ToAgentView converts one agent status.
func ToAgentView(a app.AgentStatus) AgentView {
	return AgentView{
		ID: string(a.ID), Name: a.Name, Enabled: a.Enabled,
		GlobalDirs: nonNil(a.GlobalDirs), ProjectDirs: nonNil(a.ProjectDirs),
		Exists: a.Exists, SkillCount: a.SkillCount, ReadOnly: a.ID == "claude",
	}
}

// ToSkillViews converts skills.
func ToSkillViews(skills []domain.Skill) []SkillView {
	out := make([]SkillView, 0, len(skills))
	for _, s := range skills {
		out = append(out, ToSkillView(s))
	}
	return out
}

// ToSkillView converts one skill.
func ToSkillView(s domain.Skill) SkillView {
	return SkillView{
		ID: s.ID, Agent: string(s.AgentID), Scope: string(s.Scope), ProjectRoot: s.Scope.ProjectRoot(),
		Path: s.Path, Name: s.Name, Description: s.Description, Version: s.Version, ContentHash: s.ContentHash,
		IsSymlink: s.IsSymlink, LinkTarget: s.LinkTarget, State: string(s.State), QuarantinePath: s.QuarantinePath,
		ReadOnly: s.ReadOnly, VaultRef: s.VaultRef, LastSeenAt: s.LastSeenAt.UTC().Format(time.RFC3339),
	}
}

// ToProjectViews converts projects.
func ToProjectViews(list []domain.Project) []ProjectView {
	out := make([]ProjectView, 0, len(list))
	for _, p := range list {
		out = append(out, ToProjectView(p))
	}
	return out
}

// ToProjectView converts one project.
func ToProjectView(p domain.Project) ProjectView {
	return ProjectView{ID: p.ID, Root: p.Root, Name: p.Name, RegisteredAt: p.RegisteredAt.UTC().Format(time.RFC3339)}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// VaultView is the JSON shape of a vault entry with its links.
type VaultView struct {
	Name          string             `json:"name"`
	Path          string             `json:"path"`
	ContentHash   string             `json:"contentHash"`
	Source        domain.VaultSource `json:"source"`
	SourceHash    string             `json:"sourceHash"`
	InstalledAt   string             `json:"installedAt"`
	UpdatedAt     string             `json:"updatedAt"`
	ConvertedFrom string             `json:"convertedFrom,omitempty"`
	Spec          domain.SpecReport  `json:"spec"`
	Links         []SkillView        `json:"links"`
}

// ToVaultViews converts vault statuses.
func ToVaultViews(list []app.VaultStatus) []VaultView {
	out := make([]VaultView, 0, len(list))
	for _, v := range list {
		out = append(out, ToVaultView(v))
	}
	return out
}

// ToVaultView converts one vault status.
func ToVaultView(v app.VaultStatus) VaultView {
	return VaultView{
		Name: v.Name, Path: v.Path, ContentHash: v.ContentHash, Source: v.Source, SourceHash: v.SourceHash,
		InstalledAt: v.InstalledAt.UTC().Format(time.RFC3339), UpdatedAt: v.UpdatedAt.UTC().Format(time.RFC3339),
		ConvertedFrom: v.ConvertedFrom, Spec: v.Spec, Links: ToSkillViews(v.Links),
	}
}
