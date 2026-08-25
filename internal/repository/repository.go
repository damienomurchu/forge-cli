// Package repository persists Forge domain records in SQLite.
package repository

import (
	"database/sql"
	"fmt"
)

// Repository performs record operations through one configured SQLite handle.
type Repository struct {
	db *sql.DB
}

// New constructs a repository around an initialized Forge database.
func New(db *sql.DB) (*Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("sqlite database is required")
	}
	return &Repository{db: db}, nil
}
