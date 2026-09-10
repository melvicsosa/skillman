package app

import (
	"context"
	"path/filepath"

	"github.com/melvicsosa/skillman/internal/domain"
)

// UsageReport is the telemetry read from the agent plus its origin.
type UsageReport struct {
	Source string                       `json:"source"`
	Skills map[string]domain.SkillUsage `json:"skills"`
}

// Usage returns Claude Code's skill usage telemetry. Without a configured
// source it returns an empty report.
func (s *Service) Usage(ctx context.Context) (UsageReport, error) {
	_ = ctx
	if s.usage == nil {
		return UsageReport{Skills: map[string]domain.SkillUsage{}}, nil
	}
	m, src, err := s.usage.Usage()
	if err != nil {
		return UsageReport{Source: src}, err
	}
	return UsageReport{Source: src, Skills: m}, nil
}

// UsageFor looks up the telemetry matching a discovered skill. Claude Code
// keys global and project skills by directory name; plugin skills are keyed
// "<plugin>:<name>", where the plugin is the directory three levels above
// the skill (<cache>/<marketplace>/<plugin>/<version>/skills/<name>).
func UsageFor(usage map[string]domain.SkillUsage, agent domain.AgentID, name, path string) (domain.SkillUsage, bool) {
	if len(usage) == 0 {
		return domain.SkillUsage{}, false
	}
	switch agent {
	case "claude-code":
		u, ok := usage[name]
		return u, ok
	case "claude":
		skillsDir := filepath.Dir(path)
		plugin := filepath.Base(filepath.Dir(filepath.Dir(skillsDir)))
		if u, ok := usage[plugin+":"+name]; ok {
			return u, true
		}
		u, ok := usage[name]
		return u, ok
	}
	return domain.SkillUsage{}, false
}
