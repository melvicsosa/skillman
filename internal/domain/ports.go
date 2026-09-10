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
	// ErrRejected is returned when a source cannot be converted to a skill.
	ErrRejected = errors.New("rejected")
	// ErrLinked is returned when a vault entry is still installed somewhere.
	ErrLinked = errors.New("still linked")
	// ErrAuthRequired is returned by registries whose endpoint needs a token.
	ErrAuthRequired = errors.New("authentication required")
	// ErrUnsupported is returned when a registry does not offer an operation.
	ErrUnsupported = errors.New("unsupported")
	// ErrCanceled is returned when the user dismisses an interactive prompt.
	ErrCanceled = errors.New("canceled")
)

// FolderPicker asks the user to choose a folder through a native dialog.
// It returns ErrCanceled when the dialog is dismissed and ErrUnsupported
// on platforms without a native picker.
type FolderPicker interface {
	PickFolder(ctx context.Context, prompt string) (path string, err error)
}

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

// VaultRepository caches vault entries (the provenance sidecars are the truth).
type VaultRepository interface {
	ListVault(ctx context.Context) ([]VaultEntry, error)
	// GetVault returns ErrNotFound when the entry is unknown.
	GetVault(ctx context.Context, name string) (VaultEntry, error)
	UpsertVault(ctx context.Context, e VaultEntry) error
	DeleteVault(ctx context.Context, name string) error
}

// RegistryResult is one hit of a registry search or listing.
type RegistryResult struct {
	Registry string `json:"registry"` // registry ID
	ID       string `json:"id"`       // registry-specific stable id
	Name     string `json:"name"`
	Source   string `json:"source"` // "owner/repo" or provider
	// Description is the registry's summary when it offers one.
	Description string `json:"description,omitempty"`
	Installs    int    `json:"installs,omitempty"`
	URL         string `json:"url,omitempty"`
	InstallURL  string `json:"installUrl,omitempty"`
	// Ref is the reference to pass to `add` to install this result.
	Ref string `json:"ref"`
}

// CuratedOwner groups the curated skills of one owner.
type CuratedOwner struct {
	Owner         string           `json:"owner"`
	TotalInstalls int              `json:"totalInstalls"`
	Skills        []RegistryResult `json:"skills"`
}

// FetchResult is what a registry hands back after downloading a ref: a
// temporary directory holding the fetched content and its provenance.
// Callers must call Cleanup when done.
type FetchResult struct {
	Dir     string // directory to detect/convert (a skill dir, a plugin dir, ...)
	Name    string // suggested skill name, "" when the converter should derive it
	Source  VaultSource
	Cleanup func()
}

// Registry is one source of downloadable skills.
type Registry interface {
	ID() string
	// Search returns hits for q. Registries without search return ErrUnsupported.
	Search(ctx context.Context, q string) ([]RegistryResult, error)
	// Fetch downloads ref into a temp dir.
	Fetch(ctx context.Context, ref string) (FetchResult, error)
}

// TrendingLister is implemented by registries that expose a trending list.
type TrendingLister interface {
	Trending(ctx context.Context) ([]RegistryResult, error)
}

// CuratedLister is implemented by registries that expose a curated list.
type CuratedLister interface {
	Curated(ctx context.Context) ([]CuratedOwner, error)
}

// SourceShape is what a converter detected in a source directory or file.
type SourceShape string

// Source shapes (PLAN.md section 4).
const (
	ShapeSkill           SourceShape = "skill"
	ShapeCursorRule      SourceShape = "cursor-rule"
	ShapeClaudeCommand   SourceShape = "claude-command"
	ShapeOpenCodeCommand SourceShape = "opencode-command"
	ShapeClaudePlugin    SourceShape = "claude-plugin"
	// ShapeMarketplace is a repository holding .claude-plugin/marketplace.json;
	// it is never converted, one of its plugins must be picked.
	ShapeMarketplace SourceShape = "claude-marketplace"
	ShapeUnknown     SourceShape = "unknown"
)

// ConvertedSkill is one spec skill produced by a conversion.
type ConvertedSkill struct {
	Name          string   `json:"name"`
	Path          string   `json:"path"`
	ConvertedFrom string   `json:"convertedFrom,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

// ConversionReport describes what a conversion produced.
type ConversionReport struct {
	Shape  SourceShape      `json:"shape"`
	Skills []ConvertedSkill `json:"skills"`
}

// Converter detects the shape of a source and normalizes it to spec skills.
type Converter interface {
	// Detect classifies src (a directory or a single file).
	Detect(src string) (SourceShape, error)
	// Convert writes one spec skill per produced skill under dstRoot/<name>.
	// name is the requested skill name for single-skill shapes; "" derives it
	// from src. Sources that cannot be converted return an error wrapping
	// ErrRejected.
	Convert(src, dstRoot, name string) (ConversionReport, error)
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
