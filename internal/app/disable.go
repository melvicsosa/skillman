package app

import (
	"context"
	"fmt"

	"github.com/melvicsosa/skillman/internal/domain"
)

// DisableSkill moves the skill directory (or symlink) into quarantine and
// records the new location. Disabling an already disabled skill is a no-op.
// Read-only skills (Claude plugins) cannot be disabled.
func (s *Service) DisableSkill(ctx context.Context, id string) (domain.Skill, error) {
	sk, err := s.skills.GetByID(ctx, id)
	if err != nil {
		return domain.Skill{}, err
	}
	if sk.ReadOnly {
		return sk, fmt.Errorf("%s (%s): %w", sk.Name, sk.AgentID, domain.ErrReadOnly)
	}
	if sk.State == domain.SkillDisabled {
		return sk, nil
	}
	dest, err := s.quarantine.Disable(sk.Path, s.quarantineDirFor(sk.AgentID, sk.Scope))
	if err != nil {
		return sk, err
	}
	if err := writeOrigin(dest, origin{Agent: sk.AgentID, Scope: sk.Scope, Path: sk.Path, Name: sk.Name}); err != nil {
		_ = s.quarantine.Enable(dest, sk.Path)
		return sk, fmt.Errorf("write quarantine sidecar: %w", err)
	}
	if err := s.skills.UpdateState(ctx, sk.ID, domain.SkillDisabled, dest); err != nil {
		return sk, err
	}
	sk.State = domain.SkillDisabled
	sk.QuarantinePath = dest
	return sk, nil
}
