package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/melvicsosa/skillman/internal/domain"
)

// SettingsRepo implements domain.SettingsRepository.
type SettingsRepo struct{ db *DB }

var _ domain.SettingsRepository = (*SettingsRepo)(nil)

// NewSettingsRepo returns a settings repository backed by db.
func NewSettingsRepo(db *DB) *SettingsRepo { return &SettingsRepo{db: db} }

// GetSetting returns the value for key; ok is false when the key is absent.
func (r *SettingsRepo) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get setting %q: %w", key, err)
	}
	return v, true, nil
}

// SetSetting inserts or replaces the value for key.
func (r *SettingsRepo) SetSetting(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, now())
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}
