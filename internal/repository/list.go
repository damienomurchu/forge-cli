package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// CaptureFilters contains optional AND-combined capture filters.
type CaptureFilters struct {
	Type    *domain.CaptureType
	Project *string
	Limit   *int
}

// List returns matching captures newest-first with ID
// as the deterministic descending tie-breaker.
func (r *Repository) List(
	ctx context.Context,
	filters CaptureFilters,
) ([]domain.Capture, error) {
	if err := validateCaptureFilters(filters); err != nil {
		return nil, fmt.Errorf("validate capture filters: %w", err)
	}
	query, arguments := captureListQuery(filters)
	rows, err := r.db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query captures: %w", err)
	}

	captures := make([]domain.Capture, 0)
	for rows.Next() {
		stored, err := scanStoredCapture(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan capture: %w", err)
		}
		capture, err := decodeStoredCapture(stored)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode capture: %w", err)
		}
		captures = append(captures, capture)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate captures: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close captures: %w", err)
	}
	return captures, nil
}

func validateCaptureFilters(filters CaptureFilters) error {
	if filters.Type != nil && !filters.Type.Valid() {
		return &domain.InvalidValueError{Field: "capture type", Value: filters.Type.String()}
	}
	if filters.Project != nil {
		normalized := domain.NormalizeOptionalText(*filters.Project)
		if normalized == nil || *normalized != *filters.Project {
			return &domain.InvalidValueError{Field: "project", Value: *filters.Project}
		}
	}
	if filters.Limit != nil && *filters.Limit <= 0 {
		return &domain.InvalidValueError{Field: "limit", Value: strconv.Itoa(*filters.Limit)}
	}
	return nil
}

func captureListQuery(filters CaptureFilters) (string, []any) {
	clauses := make([]string, 0, 2)
	arguments := make([]any, 0, 3)
	if filters.Type != nil {
		clauses = append(clauses, "capture_type = ?")
		arguments = append(arguments, filters.Type.String())
	}
	if filters.Project != nil {
		clauses = append(clauses, "friction_project = ?")
		arguments = append(arguments, *filters.Project)
	}

	query := `SELECT ` + captureColumns + ` FROM records`
	if len(clauses) != 0 {
		query += ` WHERE ` + strings.Join(clauses, ` AND `)
	}
	query += ` ORDER BY created_at DESC, id DESC`
	if filters.Limit != nil {
		query += ` LIMIT ?`
		arguments = append(arguments, *filters.Limit)
	}
	return query, arguments
}
