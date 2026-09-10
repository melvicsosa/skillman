package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/melvicsosa/skillman/internal/domain"
)

// SkillRepo implements domain.SkillRepository.
type SkillRepo struct{ db *DB }

var _ domain.SkillRepository = (*SkillRepo)(nil)

// NewSkillRepo returns a skill repository backed by db.
func NewSkillRepo(db *DB) *SkillRepo { return &SkillRepo{db: db} }

const skillColumns = `id, agent_id, project_id, scope, path, name, description, version,
	content_hash, is_symlink, link_target, state, quarantine_path, vault_ref, read_only, last_seen_at`

// UpsertMany inserts or fully replaces every skill in one transaction.
func (r *SkillRepo) UpsertMany(ctx context.Context, skills []domain.Skill) error {
	if len(skills) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin upsert: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO skills (`+skillColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			agent_id = excluded.agent_id,
			project_id = excluded.project_id,
			scope = excluded.scope,
			path = excluded.path,
			name = excluded.name,
			description = excluded.description,
			version = excluded.version,
			content_hash = excluded.content_hash,
			is_symlink = excluded.is_symlink,
			link_target = excluded.link_target,
			state = excluded.state,
			quarantine_path = excluded.quarantine_path,
			vault_ref = excluded.vault_ref,
			read_only = excluded.read_only,
			last_seen_at = excluded.last_seen_at`)
	if err != nil {
		return fmt.Errorf("prepare upsert: %w", err)
	}
	defer stmt.Close()
	for _, s := range skills {
		if s.ID == "" {
			s.ID = domain.SkillID(s.AgentID, s.Scope, s.Path)
		}
		if s.State == "" {
			s.State = domain.SkillEnabled
		}
		seen := s.LastSeenAt
		if seen.IsZero() {
			seen = time.Now()
		}
		var projectID any
		if s.ProjectID != nil {
			projectID = *s.ProjectID
		}
		if _, err := stmt.ExecContext(ctx, s.ID, string(s.AgentID), projectID, string(s.Scope), s.Path,
			s.Name, s.Description, s.Version, s.ContentHash, s.IsSymlink, s.LinkTarget, string(s.State),
			s.QuarantinePath, s.VaultRef, s.ReadOnly, seen.UTC().Format(time.RFC3339)); err != nil {
			return fmt.Errorf("upsert skill %s (%s): %w", s.Name, s.Path, err)
		}
	}
	return tx.Commit()
}

// List returns skills matching f, ordered by agent, scope and name.
func (r *SkillRepo) List(ctx context.Context, f domain.SkillFilter) ([]domain.Skill, error) {
	var (
		where []string
		args  []any
	)
	if f.Agent != "" {
		where = append(where, "agent_id = ?")
		args = append(args, string(f.Agent))
	}
	if f.Scope != "" {
		where = append(where, "scope = ?")
		args = append(args, string(f.Scope))
	}
	if f.State != "" {
		where = append(where, "state = ?")
		args = append(args, string(f.State))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, "(name LIKE ? ESCAPE '\\' OR description LIKE ? ESCAPE '\\')")
		pat := "%" + escapeLike(strings.ToLower(q)) + "%"
		args = append(args, pat, pat)
	}
	query := `SELECT ` + skillColumns + ` FROM skills`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY agent_id, scope, name, path"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()
	out := []domain.Skill{}
	for rows.Next() {
		s, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetByID returns the skill or domain.ErrNotFound.
func (r *SkillRepo) GetByID(ctx context.Context, id string) (domain.Skill, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+skillColumns+` FROM skills WHERE id = ?`, id)
	s, err := scanSkill(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Skill{}, domain.ErrNotFound
	}
	return s, err
}

// UpdateState sets the state and quarantine path of one skill.
func (r *SkillRepo) UpdateState(ctx context.Context, id string, state domain.SkillState, quarantinePath string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE skills SET state = ?, quarantine_path = ? WHERE id = ?`,
		string(state), quarantinePath, id)
	if err != nil {
		return fmt.Errorf("update skill state: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// MarkMissing deletes non-disabled rows of (agent, scope) not present in seen.
func (r *SkillRepo) MarkMissing(ctx context.Context, agent domain.AgentID, scope domain.Scope, seen []string) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin mark missing: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE IF NOT EXISTS seen_ids (id TEXT PRIMARY KEY)`); err != nil {
		return 0, fmt.Errorf("create seen table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM seen_ids`); err != nil {
		return 0, err
	}
	for _, id := range seen {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO seen_ids (id) VALUES (?)`, id); err != nil {
			return 0, err
		}
	}
	res, err := tx.ExecContext(ctx, `
		DELETE FROM skills
		WHERE agent_id = ? AND scope = ? AND state <> ?
		  AND id NOT IN (SELECT id FROM seen_ids)`,
		string(agent), string(scope), string(domain.SkillDisabled))
	if err != nil {
		return 0, fmt.Errorf("delete missing skills: %w", err)
	}
	n, _ := res.RowsAffected()
	if _, err := tx.ExecContext(ctx, `DELETE FROM seen_ids`); err != nil {
		return 0, err
	}
	return int(n), tx.Commit()
}

type scanner interface{ Scan(dest ...any) error }

func scanSkill(row scanner) (domain.Skill, error) {
	var (
		s         domain.Skill
		projectID sql.NullInt64
		seen      string
	)
	if err := row.Scan(&s.ID, &s.AgentID, &projectID, &s.Scope, &s.Path, &s.Name, &s.Description, &s.Version,
		&s.ContentHash, &s.IsSymlink, &s.LinkTarget, &s.State, &s.QuarantinePath, &s.VaultRef, &s.ReadOnly, &seen); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return s, err
		}
		return s, fmt.Errorf("scan skill: %w", err)
	}
	if projectID.Valid {
		v := projectID.Int64
		s.ProjectID = &v
	}
	s.LastSeenAt, _ = time.Parse(time.RFC3339, seen)
	return s, nil
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
