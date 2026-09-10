// Package app holds the use cases. Every CLI command and HTTP route goes
// through a Service method so both surfaces behave identically.
package app

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"path/filepath"

	"github.com/melvicsosa/skillman/internal/domain"
)

// QuarantineDirName is the subdirectory of the data dir holding disabled skills.
const QuarantineDirName = "disabled"

// Service wires the ports together. Construct it with New.
type Service struct {
	agents     domain.AgentSource
	agentRepo  domain.AgentRepository
	skills     domain.SkillRepository
	projects   domain.ProjectRepository
	scans      domain.ScanRepository
	inspector  domain.SkillInspector
	quarantine domain.Quarantine
	vault      domain.VaultRepository
	settings   domain.SettingsRepository
	converter  domain.Converter
	registries []domain.Registry
	dataDir    string
}

// Deps lists everything a Service needs.
type Deps struct {
	Agents     domain.AgentSource
	AgentRepo  domain.AgentRepository
	Skills     domain.SkillRepository
	Projects   domain.ProjectRepository
	Scans      domain.ScanRepository
	Inspector  domain.SkillInspector
	Quarantine domain.Quarantine
	Vault      domain.VaultRepository
	Settings   domain.SettingsRepository
	Converter  domain.Converter
	// Registries are consulted in order by ResolveRef; the first whose
	// Matches (see RefMatcher) accepts the ref wins.
	Registries []domain.Registry
	DataDir    string
}

// New builds a Service from its dependencies.
func New(d Deps) *Service {
	return &Service{
		agents: d.Agents, agentRepo: d.AgentRepo, skills: d.Skills, projects: d.Projects,
		scans: d.Scans, inspector: d.Inspector, quarantine: d.Quarantine, vault: d.Vault, settings: d.Settings,
		converter: d.Converter, registries: d.Registries, dataDir: d.DataDir,
	}
}

// QuarantineRoot is <dataDir>/disabled.
func (s *Service) QuarantineRoot() string { return filepath.Join(s.dataDir, QuarantineDirName) }

// quarantineDirFor returns where disabled skills of (agent, scope) live:
// disabled/<agent> for global skills and disabled/<agent>/projects/<hash>
// for project skills, so equally named skills never collide.
func (s *Service) quarantineDirFor(agent domain.AgentID, scope domain.Scope) string {
	dir := filepath.Join(s.QuarantineRoot(), string(agent))
	if root := scope.ProjectRoot(); root != "" {
		sum := sha1.Sum([]byte(root))
		dir = filepath.Join(dir, "projects", hex.EncodeToString(sum[:])[:12])
	}
	return dir
}

// ensureAgentRows makes sure every registry agent has a row so skills can
// reference it. Existing rows (and their enabled flag) are left untouched.
func (s *Service) ensureAgentRows(ctx context.Context) error {
	existing, err := s.agentRepo.ListAgents(ctx)
	if err != nil {
		return err
	}
	known := map[domain.AgentID]bool{}
	for _, a := range existing {
		known[a.ID] = true
	}
	for _, a := range s.agents.Agents() {
		if known[a.ID] {
			continue
		}
		if err := s.agentRepo.UpsertAgent(ctx, a); err != nil {
			return err
		}
	}
	return nil
}
