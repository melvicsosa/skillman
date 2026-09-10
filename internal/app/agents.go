package app

import (
	"context"
	"fmt"

	"github.com/melvicsosa/skillman/internal/domain"
)

// AgentStatus is an agent with its persisted enabled flag and live facts.
type AgentStatus struct {
	domain.Agent
	Exists     bool
	SkillCount int
}

// ListAgents returns every supported agent in registry order, merged with
// the persisted enabled flag (default true) and its skill count.
func (s *Service) ListAgents(ctx context.Context) ([]AgentStatus, error) {
	persisted, err := s.agentRepo.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	enabled := map[domain.AgentID]bool{}
	for _, a := range persisted {
		enabled[a.ID] = a.Enabled
	}
	counts := map[domain.AgentID]int{}
	skills, err := s.skills.List(ctx, domain.SkillFilter{})
	if err != nil {
		return nil, err
	}
	for _, sk := range skills {
		counts[sk.AgentID]++
	}
	out := make([]AgentStatus, 0, len(s.agents.Agents()))
	for _, a := range s.agents.Agents() {
		if v, ok := enabled[a.ID]; ok {
			a.Enabled = v
		}
		out = append(out, AgentStatus{Agent: a, Exists: s.agents.Exists(a), SkillCount: counts[a.ID]})
	}
	return out, nil
}

// enabledAgents returns the agents whose persisted flag is on.
func (s *Service) enabledAgents(ctx context.Context) ([]domain.Agent, error) {
	all, err := s.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	var out []domain.Agent
	for _, a := range all {
		if a.Enabled {
			out = append(out, a.Agent)
		}
	}
	return out, nil
}

// ToggleAgent persists the enabled flag of one agent. No files are touched.
func (s *Service) ToggleAgent(ctx context.Context, id domain.AgentID, enabled bool) (domain.Agent, error) {
	for _, a := range s.agents.Agents() {
		if a.ID != id {
			continue
		}
		a.Enabled = enabled
		if err := s.agentRepo.UpsertAgent(ctx, a); err != nil {
			return domain.Agent{}, err
		}
		return a, nil
	}
	return domain.Agent{}, fmt.Errorf("agent %q: %w", id, domain.ErrNotFound)
}
