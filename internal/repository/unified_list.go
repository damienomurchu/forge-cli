package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// UnifiedCaptureFilters contains optional AND-combined schema-2 filters.
type UnifiedCaptureFilters struct {
	Type    *domain.CaptureType
	Project *string
	Limit   *int
}

// ListUnifiedCaptures returns matching schema-2 captures newest-first with ID
// as the deterministic descending tie-breaker.
func (r *Repository) ListUnifiedCaptures(
	ctx context.Context,
	filters UnifiedCaptureFilters,
) ([]domain.Capture, error) {
	if err := validateUnifiedCaptureFilters(filters); err != nil {
		return nil, fmt.Errorf("validate unified capture filters: %w", err)
	}
	query, arguments := unifiedCaptureListQuery(filters)
	rows, err := r.db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query unified captures: %w", err)
	}

	captures := make([]domain.Capture, 0)
	for rows.Next() {
		stored, err := scanStoredUnifiedCapture(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan unified capture: %w", err)
		}
		capture, err := decodeStoredUnifiedCapture(stored)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode unified capture: %w", err)
		}
		captures = append(captures, capture)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate unified captures: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close unified captures: %w", err)
	}
	return captures, nil
}

func validateUnifiedCaptureFilters(filters UnifiedCaptureFilters) error {
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

func unifiedCaptureListQuery(filters UnifiedCaptureFilters) (string, []any) {
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

	query := `SELECT ` + unifiedCaptureColumns + ` FROM records`
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
