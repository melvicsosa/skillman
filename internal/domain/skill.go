package domain

import "time"

// SkillState is the enabled/disabled state of a discovered skill.
type SkillState string

const (
	SkillEnabled  SkillState = "enabled"
	SkillDisabled SkillState = "disabled"
)

// Scope is either "global" or "project:<root>".
type Scope string

// GlobalScope is the scope of skills installed in an agent's global directory.
const GlobalScope Scope = "global"

// Skill is one skill directory discovered in an agent's skills location.
// Identity is (AgentID, Scope, Path).
type Skill struct {
	ID             int64
	AgentID        AgentID
	Scope          Scope
	ProjectID      *int64
	Path           string
	Name           string
	Description    string
	Version        string
	ContentHash    string
	IsSymlink      bool
	LinkTarget     string
	State          SkillState
	QuarantinePath string
	VaultRef       string
	LastSeenAt     time.Time
}

// VaultSource records where a vault entry was downloaded from.
type VaultSource struct {
	Type    string // "skillssh", "github", "local", "url"
	Ref     string
	URL     string
	Subpath string
}

// SpecReport is the result of validating a skill against the Agent Skills spec.
type SpecReport struct {
	Valid  bool
	Issues []string
}

// VaultEntry is one canonical local copy of a downloaded skill.
type VaultEntry struct {
	Name        string
	Path        string
	ContentHash string
	Source      VaultSource
	SourceHash  string
	InstalledAt time.Time
	UpdatedAt   time.Time
	Spec        SpecReport
}
