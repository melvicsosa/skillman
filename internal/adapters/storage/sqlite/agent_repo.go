package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/melvicsosa/skillman/internal/domain"
)

// AgentRepo implements domain.AgentRepository.
type AgentRepo struct{ db *DB }

var _ domain.AgentRepository = (*AgentRepo)(nil)

// NewAgentRepo returns an agent repository backed by db.
func NewAgentRepo(db *DB) *AgentRepo { return &AgentRepo{db: db} }

// ListAgents returns every persisted agent ordered by id.
func (r *AgentRepo) ListAgents(ctx context.Context) ([]domain.Agent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, enabled, global_dirs, project_dirs, supports_symlink
		FROM agents ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var out []domain.Agent
	for rows.Next() {
		var (
			a                    domain.Agent
			globalJSON, projJSON string
		)
		if err := rows.Scan(&a.ID, &a.Name, &a.Enabled, &globalJSON, &projJSON, &a.SupportsSymlink); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		if err := json.Unmarshal([]byte(globalJSON), &a.GlobalDirs); err != nil {
			return nil, fmt.Errorf("agent %s global_dirs: %w", a.ID, err)
		}
		if err := json.Unmarshal([]byte(projJSON), &a.ProjectDirs); err != nil {
			return nil, fmt.Errorf("agent %s project_dirs: %w", a.ID, err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpsertAgent inserts the agent or updates every mutable column.
func (r *AgentRepo) UpsertAgent(ctx context.Context, a domain.Agent) error {
	globalJSON, err := json.Marshal(nonNil(a.GlobalDirs))
	if err != nil {
		return err
	}
	projJSON, err := json.Marshal(nonNil(a.ProjectDirs))
	if err != nil {
		return err
	}
	ts := now()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO agents (id, name, enabled, global_dirs, project_dirs, supports_symlink, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			enabled = excluded.enabled,
			global_dirs = excluded.global_dirs,
			project_dirs = excluded.project_dirs,
			supports_symlink = excluded.supports_symlink,
			updated_at = excluded.updated_at`,
		string(a.ID), a.Name, a.Enabled, string(globalJSON), string(projJSON), a.SupportsSymlink, ts, ts)
	if err != nil {
		return fmt.Errorf("upsert agent %s: %w", a.ID, err)
	}
	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
