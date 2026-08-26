// Package repository persists Forge domain records in SQLite.
package repository

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrRecordNotFound identifies a lookup for an ID that is not stored.
var ErrRecordNotFound = errors.New("record not found")

type rowScanner interface {
	Scan(dest ...any) error
}

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

func optionalString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func stringPointerFromNull(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
