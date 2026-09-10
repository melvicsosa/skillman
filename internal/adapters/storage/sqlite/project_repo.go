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

// ProjectRepo implements domain.ProjectRepository.
type ProjectRepo struct{ db *DB }

var _ domain.ProjectRepository = (*ProjectRepo)(nil)

// NewProjectRepo returns a project repository backed by db.
func NewProjectRepo(db *DB) *ProjectRepo { return &ProjectRepo{db: db} }

// Add registers p and returns it with its ID. Registering an existing root
// returns domain.ErrExists.
func (r *ProjectRepo) Add(ctx context.Context, p domain.Project) (domain.Project, error) {
	if p.RegisteredAt.IsZero() {
		p.RegisteredAt = time.Now()
	}
	res, err := r.db.ExecContext(ctx, `INSERT INTO projects (root, name, registered_at) VALUES (?, ?, ?)`,
		p.Root, p.Name, p.RegisteredAt.UTC().Format(time.RFC3339))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return domain.Project{}, fmt.Errorf("project %s: %w", p.Root, domain.ErrExists)
		}
		return domain.Project{}, fmt.Errorf("add project: %w", err)
	}
	p.ID, err = res.LastInsertId()
	return p, err
}

// List returns every registered project ordered by root.
func (r *ProjectRepo) List(ctx context.Context) ([]domain.Project, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, root, name, registered_at FROM projects ORDER BY root`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	out := []domain.Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Remove deletes the project (and, via cascade, its skills).
func (r *ProjectRepo) Remove(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("remove project: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// GetByRoot returns the project registered at root or domain.ErrNotFound.
func (r *ProjectRepo) GetByRoot(ctx context.Context, root string) (domain.Project, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, root, name, registered_at FROM projects WHERE root = ?`, root)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Project{}, domain.ErrNotFound
	}
	return p, err
}

func scanProject(row scanner) (domain.Project, error) {
	var (
		p  domain.Project
		ts string
	)
	if err := row.Scan(&p.ID, &p.Root, &p.Name, &ts); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return p, err
		}
		return p, fmt.Errorf("scan project: %w", err)
	}
	p.RegisteredAt, _ = time.Parse(time.RFC3339, ts)
	return p, nil
}
