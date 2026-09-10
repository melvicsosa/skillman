package app

import (
	"context"
	"fmt"
	"os"

	"github.com/melvicsosa/skillman/internal/domain"
)

// EnableSkill moves a quarantined skill back to its original path.
// Enabling an already enabled skill is a no-op.
func (s *Service) EnableSkill(ctx context.Context, id string) (domain.Skill, error) {
	sk, err := s.skills.GetByID(ctx, id)
	if err != nil {
		return domain.Skill{}, err
	}
	if sk.State == domain.SkillEnabled {
		return sk, nil
	}
	if sk.QuarantinePath == "" {
		return sk, fmt.Errorf("skill %s is disabled but has no quarantine path; rescan", sk.Name)
	}
	if err := s.quarantine.Enable(sk.QuarantinePath, sk.Path); err != nil {
		return sk, err
	}
	_ = os.Remove(sidecarPath(sk.QuarantinePath))
	if err := s.skills.UpdateState(ctx, sk.ID, domain.SkillEnabled, ""); err != nil {
		return sk, err
	}
	sk.State = domain.SkillEnabled
	sk.QuarantinePath = ""
	return sk, nil
}
