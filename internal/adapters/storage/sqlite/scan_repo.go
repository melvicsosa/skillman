package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/melvicsosa/skillman/internal/domain"
)

// ScanRepo implements domain.ScanRepository.
type ScanRepo struct{ db *DB }

var _ domain.ScanRepository = (*ScanRepo)(nil)

// NewScanRepo returns a scan history repository backed by db.
func NewScanRepo(db *DB) *ScanRepo { return &ScanRepo{db: db} }

// Start records the beginning of a scan and returns its row ID.
func (r *ScanRepo) Start(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO scans (started_at) VALUES (?)`, now())
	if err != nil {
		return 0, fmt.Errorf("start scan: %w", err)
	}
	return res.LastInsertId()
}

// Finish records the end of a scan with its totals.
func (r *ScanRepo) Finish(ctx context.Context, id int64, skillsFound int, errs []string) error {
	if errs == nil {
		errs = []string{}
	}
	errJSON, err := json.Marshal(errs)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE scans SET finished_at = ?, skills_found = ?, errors = ? WHERE id = ?`,
		now(), skillsFound, string(errJSON), id)
	if err != nil {
		return fmt.Errorf("finish scan: %w", err)
	}
	return nil
}
