package domain

// AgentID is the stable slug identifying an agent (for example "claude-code").
type AgentID string

// Agent describes one AI coding agent and where it looks for skills.
type Agent struct {
	ID              AgentID
	Name            string
	Enabled         bool
	GlobalDirs      []string // absolute paths, first entry is the primary location
	ProjectDirs     []string // relative to a project root, first entry is the primary location
	SupportsSymlink bool
}
