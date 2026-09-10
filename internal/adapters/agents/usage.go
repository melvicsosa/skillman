package agents

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/melvicsosa/skillman/internal/domain"
)

// ClaudeUsage reads Claude Code's skill telemetry from ~/.claude.json. The
// file is a map keyed by skill name (plugin skills use "<plugin>:<skill>")
// with usageCount and lastUsedAt (Unix milliseconds):
//
//	"skillUsage": {"branch-pr": {"usageCount": 10, "lastUsedAt": 1788913317724}}
type ClaudeUsage struct{}

var _ domain.UsageSource = ClaudeUsage{}

// ClaudeConfigPath is the ~/.claude.json location, honouring SKILLMAN_HOME.
func ClaudeConfigPath() string { return ExpandHome("~/.claude.json") }

// Usage implements domain.UsageSource.
func (ClaudeUsage) Usage() (map[string]domain.SkillUsage, string, error) {
	p := ClaudeConfigPath()
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]domain.SkillUsage{}, p, nil
	}
	if err != nil {
		return nil, p, fmt.Errorf("read %s: %w", p, err)
	}
	var doc struct {
		SkillUsage map[string]struct {
			UsageCount int   `json:"usageCount"`
			LastUsedAt int64 `json:"lastUsedAt"`
		} `json:"skillUsage"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, p, fmt.Errorf("parse %s: %w", filepath.Base(p), err)
	}
	out := make(map[string]domain.SkillUsage, len(doc.SkillUsage))
	for k, v := range doc.SkillUsage {
		u := domain.SkillUsage{UsageCount: v.UsageCount}
		if v.LastUsedAt > 0 {
			u.LastUsedAt = time.UnixMilli(v.LastUsedAt).UTC()
		}
		out[k] = u
	}
	return out, p, nil
}
