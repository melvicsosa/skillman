// Package agents holds the static registry of supported agents and the
// directories each one reads skills from (PLAN.md section 1.2).
package agents

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/melvicsosa/skillman/internal/domain"
)

// Spec is the static description of one supported agent. Paths starting
// with "~" are expanded relative to the current user's home at runtime.
type Spec struct {
	ID          domain.AgentID
	Name        string
	GlobalDirs  []string // first entry is the primary location
	ProjectDirs []string // relative to a project root, first entry is primary
}

// Registry is the ordered list of supported agents.
var Registry = []Spec{
	{
		ID: "claude-code", Name: "Claude Code",
		GlobalDirs:  []string{"~/.claude/skills"},
		ProjectDirs: []string{".claude/skills"},
	},
	{
		ID: "claude", Name: "Claude (plugins)",
		GlobalDirs:  []string{"~/.claude/plugins/cache"},
		ProjectDirs: []string{},
	},
	{
		ID: "codex", Name: "Codex",
		GlobalDirs:  []string{"~/.agents/skills", "~/.codex/skills"},
		ProjectDirs: []string{".agents/skills", ".codex/skills"},
	},
	{
		ID: "cursor", Name: "Cursor",
		GlobalDirs:  []string{"~/.cursor/skills", "~/.agents/skills"},
		ProjectDirs: []string{".cursor/skills", ".agents/skills", ".claude/skills", ".codex/skills"},
	},
	{
		ID: "gemini-cli", Name: "Gemini CLI",
		GlobalDirs:  []string{"~/.gemini/skills", "~/.agents/skills"},
		ProjectDirs: []string{".gemini/skills", ".agents/skills"},
	},
	{
		ID: "antigravity", Name: "Antigravity",
		GlobalDirs:  []string{"~/.gemini/config/skills", "~/.gemini/antigravity/skills"},
		ProjectDirs: []string{".agents/skills"},
	},
	{
		ID: "opencode", Name: "OpenCode",
		GlobalDirs:  []string{"~/.config/opencode/skills"},
		ProjectDirs: []string{".opencode/skills", ".claude/skills", ".agents/skills"},
	},
}

// Lookup returns the spec for id.
func Lookup(id domain.AgentID) (Spec, bool) {
	for _, s := range Registry {
		if s.ID == id {
			return s, true
		}
	}
	return Spec{}, false
}

// ExpandHome replaces a leading "~" with the user's home directory.
func ExpandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}

// ResolvedGlobalDirs returns the agent's global dirs with "~" expanded.
func (s Spec) ResolvedGlobalDirs() []string {
	out := make([]string, 0, len(s.GlobalDirs))
	for _, d := range s.GlobalDirs {
		out = append(out, ExpandHome(d))
	}
	return out
}

// Exists reports whether at least one of the agent's global dirs exists on
// this machine (symlinks are followed).
func (s Spec) Exists() bool {
	for _, d := range s.ResolvedGlobalDirs() {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// ToDomain converts the spec into a domain.Agent with expanded paths.
func (s Spec) ToDomain() domain.Agent {
	return domain.Agent{
		ID:              s.ID,
		Name:            s.Name,
		Enabled:         true,
		GlobalDirs:      s.ResolvedGlobalDirs(),
		ProjectDirs:     append([]string(nil), s.ProjectDirs...),
		SupportsSymlink: true,
	}
}
