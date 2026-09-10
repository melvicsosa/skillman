package domain

import "context"

// AgentRepository persists agent enabled flags and metadata.
type AgentRepository interface {
	ListAgents(ctx context.Context) ([]Agent, error)
	UpsertAgent(ctx context.Context, a Agent) error
}

// SkillRepository persists the cached view of discovered skills.
type SkillRepository interface {
	ListSkills(ctx context.Context, agent AgentID, scope Scope) ([]Skill, error)
	UpsertSkill(ctx context.Context, s Skill) error
}

// SettingsRepository stores simple key/value settings (port, github token, ...).
type SettingsRepository interface {
	GetSetting(ctx context.Context, key string) (value string, ok bool, err error)
	SetSetting(ctx context.Context, key, value string) error
}
