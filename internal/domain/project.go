package domain

import "time"

// Project is a registered project root whose per-project skill dirs are scanned.
type Project struct {
	ID           int64
	Root         string
	Name         string
	RegisteredAt time.Time
}
