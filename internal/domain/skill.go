package domain

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
	"time"
)

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

const projectScopePrefix = "project:"

// ProjectScope builds the scope for a project root.
func ProjectScope(root string) Scope { return Scope(projectScopePrefix + root) }

// IsProject reports whether the scope refers to a project.
func (s Scope) IsProject() bool { return strings.HasPrefix(string(s), projectScopePrefix) }

// ProjectRoot returns the project root for a project scope, or "" for global.
func (s Scope) ProjectRoot() string {
	if !s.IsProject() {
		return ""
	}
	return strings.TrimPrefix(string(s), projectScopePrefix)
}

// SkillID derives the stable identifier of a skill from its identity triple.
// It is the hex sha1 of "agent|scope|path", so IDs survive rescans.
func SkillID(agent AgentID, scope Scope, path string) string {
	sum := sha1.Sum([]byte(string(agent) + "|" + string(scope) + "|" + path))
	return hex.EncodeToString(sum[:])
}

// Skill is one skill directory discovered in an agent's skills location.
// Identity is (AgentID, Scope, Path); ID is derived from it via SkillID.
type Skill struct {
	ID             string
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
	ReadOnly       bool
	LastSeenAt     time.Time
}

// DiscoveredSkill is a skill directory found on disk before it is inspected.
type DiscoveredSkill struct {
	Name       string
	Path       string
	IsSymlink  bool
	LinkTarget string
	Dangling   bool // symlink whose target does not exist
	ReadOnly   bool // managed by the agent itself (for example Claude plugins)
}

// SkillInfo is what inspecting a skill directory yields: parsed frontmatter
// fields, the content hash and any spec issues found.
type SkillInfo struct {
	Name        string // frontmatter name, "" when absent
	Description string
	Version     string
	ContentHash string
	SpecIssues  []string
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
