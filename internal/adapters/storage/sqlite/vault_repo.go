package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/melvicsosa/skillman/internal/domain"
)

// VaultRepo implements domain.VaultRepository.
type VaultRepo struct{ db *DB }

var _ domain.VaultRepository = (*VaultRepo)(nil)

// NewVaultRepo returns a vault repository backed by db.
func NewVaultRepo(db *DB) *VaultRepo { return &VaultRepo{db: db} }

const vaultColumns = `name, path, content_hash, source_type, source_ref, source_url, source_subpath,
	source_commit, source_hash, converted_from, spec_valid, spec_issues, installed_at, updated_at, auto_sync`

// ListVault returns every entry ordered by name.
func (r *VaultRepo) ListVault(ctx context.Context) ([]domain.VaultEntry, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+vaultColumns+` FROM vault ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list vault: %w", err)
	}
	defer rows.Close()
	out := []domain.VaultEntry{}
	for rows.Next() {
		e, err := scanVault(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetVault returns the entry or domain.ErrNotFound.
func (r *VaultRepo) GetVault(ctx context.Context, name string) (domain.VaultEntry, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+vaultColumns+` FROM vault WHERE name = ?`, name)
	e, err := scanVault(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.VaultEntry{}, domain.ErrNotFound
	}
	return e, err
}

// UpsertVault inserts or fully replaces the entry named e.Name.
func (r *VaultRepo) UpsertVault(ctx context.Context, e domain.VaultEntry) error {
	issues, err := json.Marshal(nonNilStrings(e.Spec.Issues))
	if err != nil {
		return err
	}
	installed := e.InstalledAt
	if installed.IsZero() {
		installed = time.Now()
	}
	updated := e.UpdatedAt
	if updated.IsZero() {
		updated = installed
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO vault (`+vaultColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			path = excluded.path,
			content_hash = excluded.content_hash,
			source_type = excluded.source_type,
			source_ref = excluded.source_ref,
			source_url = excluded.source_url,
			source_subpath = excluded.source_subpath,
			source_commit = excluded.source_commit,
			source_hash = excluded.source_hash,
			converted_from = excluded.converted_from,
			spec_valid = excluded.spec_valid,
			spec_issues = excluded.spec_issues,
			installed_at = excluded.installed_at,
			updated_at = excluded.updated_at,
			auto_sync = excluded.auto_sync`,
		e.Name, e.Path, e.ContentHash, e.Source.Type, e.Source.Ref, e.Source.URL, e.Source.Subpath,
		e.Source.Commit, e.SourceHash, e.ConvertedFrom, e.Spec.Valid, string(issues),
		installed.UTC().Format(time.RFC3339), updated.UTC().Format(time.RFC3339), e.AutoSync)
	if err != nil {
		return fmt.Errorf("upsert vault %s: %w", e.Name, err)
	}
	return nil
}

// DeleteVault removes the entry; a missing entry is not an error.
func (r *VaultRepo) DeleteVault(ctx context.Context, name string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM vault WHERE name = ?`, name); err != nil {
		return fmt.Errorf("delete vault %s: %w", name, err)
	}
	return nil
}

func scanVault(row scanner) (domain.VaultEntry, error) {
	var (
		e                  domain.VaultEntry
		issues             string
		installed, updated string
	)
	if err := row.Scan(&e.Name, &e.Path, &e.ContentHash, &e.Source.Type, &e.Source.Ref, &e.Source.URL,
		&e.Source.Subpath, &e.Source.Commit, &e.SourceHash, &e.ConvertedFrom, &e.Spec.Valid, &issues,
		&installed, &updated, &e.AutoSync); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return e, err
		}
		return e, fmt.Errorf("scan vault: %w", err)
	}
	if err := json.Unmarshal([]byte(issues), &e.Spec.Issues); err != nil {
		return e, fmt.Errorf("vault %s: bad spec_issues: %w", e.Name, err)
	}
	e.Spec.Issues = nonNilStrings(e.Spec.Issues)
	e.InstalledAt, _ = time.Parse(time.RFC3339, installed)
	e.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
	return e, nil
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
