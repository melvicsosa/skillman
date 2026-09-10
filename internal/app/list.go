package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/melvicsosa/skillman/internal/domain"
)

// ListSkills returns the cached skills matching f.
func (s *Service) ListSkills(ctx context.Context, f domain.SkillFilter) ([]domain.Skill, error) {
	return s.skills.List(ctx, f)
}

// GetSkill returns one skill by ID.
func (s *Service) GetSkill(ctx context.Context, id string) (domain.Skill, error) {
	return s.skills.GetByID(ctx, id)
}

// AmbiguousError lists the skills that matched a name lookup.
type AmbiguousError struct {
	Name       string
	Candidates []domain.Skill
}

func (e *AmbiguousError) Error() string {
	paths := make([]string, 0, len(e.Candidates))
	for _, c := range e.Candidates {
		paths = append(paths, c.Path)
	}
	return fmt.Sprintf("skill %q is ambiguous, candidates: %s", e.Name, strings.Join(paths, ", "))
}

// Unwrap lets errors.Is match domain.ErrAmbiguous.
func (e *AmbiguousError) Unwrap() error { return domain.ErrAmbiguous }

// ResolveSkill finds a skill by directory name within an agent and scope.
// projectRoot selects the project scope; empty means global.
func (s *Service) ResolveSkill(ctx context.Context, name string, agent domain.AgentID, projectRoot string) (domain.Skill, error) {
	scope := domain.GlobalScope
	if projectRoot != "" {
		root, err := cleanRoot(projectRoot)
		if err != nil {
			return domain.Skill{}, err
		}
		scope = domain.ProjectScope(root)
	}
	all, err := s.skills.List(ctx, domain.SkillFilter{Agent: agent, Scope: scope})
	if err != nil {
		return domain.Skill{}, err
	}
	var matches []domain.Skill
	for _, sk := range all {
		if sk.Name == name {
			matches = append(matches, sk)
		}
	}
	switch len(matches) {
	case 0:
		return domain.Skill{}, fmt.Errorf("skill %q for agent %s in scope %s: %w", name, agent, scope, domain.ErrNotFound)
	case 1:
		return matches[0], nil
	default:
		return domain.Skill{}, &AmbiguousError{Name: name, Candidates: matches}
	}
}
