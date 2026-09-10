package domain

import (
	"context"
	"errors"
)

// Sentinel errors returned by ports and use cases.
var (
	ErrNotFound  = errors.New("not found")
	ErrReadOnly  = errors.New("skill is read-only and cannot be disabled")
	ErrAmbiguous = errors.New("ambiguous")
	ErrExists    = errors.New("already exists")
)

// SkillFilter narrows a skill listing. Zero values mean "no constraint".
type SkillFilter struct {
	Agent AgentID
	Scope Scope
	State SkillState
	Query string // case-insensitive substring of name or description
}

// AgentRepository persists agent enabled flags and metadata.
type AgentRepository interface {
	ListAgents(ctx context.Context) ([]Agent, error)
	UpsertAgent(ctx context.Context, a Agent) error
}

// SkillRepository persists the cached view of discovered skills.
type SkillRepository interface {
	// UpsertMany inserts or fully replaces every skill by ID.
	UpsertMany(ctx context.Context, skills []Skill) error
	List(ctx context.Context, f SkillFilter) ([]Skill, error)
	// GetByID returns ErrNotFound when the skill is unknown.
	GetByID(ctx context.Context, id string) (Skill, error)
	UpdateState(ctx context.Context, id string, state SkillState, quarantinePath string) error
	// MarkMissing deletes rows of (agent, scope) whose ID is not in seen,
	// except rows in the disabled state. It returns the number of rows removed.
	MarkMissing(ctx context.Context, agent AgentID, scope Scope, seen []string) (int, error)
}

// ProjectRepository persists registered project roots.
type ProjectRepository interface {
	Add(ctx context.Context, p Project) (Project, error)
	List(ctx context.Context) ([]Project, error)
	Remove(ctx context.Context, id int64) error
	// GetByRoot returns ErrNotFound when the root is not registered.
	GetByRoot(ctx context.Context, root string) (Project, error)
}

// ScanRepository records scan history.
type ScanRepository interface {
	Start(ctx context.Context) (int64, error)
	Finish(ctx context.Context, id int64, skillsFound int, errs []string) error
}

// SettingsRepository stores simple key/value settings (port, github token, ...).
type SettingsRepository interface {
	GetSetting(ctx context.Context, key string) (value string, ok bool, err error)
	SetSetting(ctx context.Context, key, value string) error
}

// AgentSource knows the supported agents and how to discover their skills.
type AgentSource interface {
	// Agents returns every supported agent in display order, with resolved
	// global dirs. Enabled is always true; the repository holds the flag.
	Agents() []Agent
	// Discover lists skill directories of agent in scope. projectRoot is
	// required for project scopes and ignored for the global scope.
	Discover(agent Agent, scope Scope, projectRoot string) ([]DiscoveredSkill, error)
	// ProjectDirs returns the absolute per-project skill dirs of agent under root.
	ProjectDirs(agent Agent, root string) []string
	// Exists reports whether at least one of the agent's global dirs exists.
	Exists(agent Agent) bool
}

// SkillInspector parses and hashes a skill directory.
type SkillInspector interface {
	Inspect(path string) (SkillInfo, error)
}

// Quarantine moves skill directories in and out of the disabled store.
type Quarantine interface {
	// Disable moves src into quarantineDir and returns the new location.
	Disable(src, quarantineDir string) (string, error)
	// Enable moves a quarantined entry back to originalPath.
	Enable(quarantined, originalPath string) error
}
