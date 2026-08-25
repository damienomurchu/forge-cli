package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// ListOptions contains optional AND-combined record filters.
type ListOptions struct {
	Type    *domain.RecordType
	Project *string
	Status  *domain.Status
	Limit   *int
}

// List returns matching records newest-first with ID as the deterministic
// descending tie-breaker. It does not modify the database.
func (r *Repository) List(ctx context.Context, options ListOptions) ([]domain.Record, error) {
	if err := validateListOptions(options); err != nil {
		return nil, fmt.Errorf("validate list options: %w", err)
	}
	query, arguments := listQuery(options)
	records, err := r.readRecords(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list records: %w", err)
	}
	return records, nil
}

func (r *Repository) readRecords(ctx context.Context, query string, arguments ...any) ([]domain.Record, error) {
	rows, err := r.db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query records: %w", err)
	}

	storedRecords := make([]storedRecord, 0)
	for rows.Next() {
		stored, err := scanStoredRecord(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan record: %w", err)
		}
		storedRecords = append(storedRecords, stored)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate records: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close records: %w", err)
	}

	records := make([]domain.Record, 0, len(storedRecords))
	for _, stored := range storedRecords {
		record, err := r.decodeStoredRecord(ctx, stored)
		if err != nil {
			return nil, fmt.Errorf("decode record: %w", err)
		}
		records = append(records, record)
	}
	return records, nil
}

func validateListOptions(options ListOptions) error {
	if options.Type != nil && !options.Type.Valid() {
		return &domain.InvalidValueError{Field: "record type", Value: options.Type.String()}
	}
	if options.Project != nil {
		normalized := domain.NormalizeOptionalText(*options.Project)
		if normalized == nil || *normalized != *options.Project {
			return &domain.InvalidValueError{Field: "project", Value: *options.Project}
		}
	}
	if options.Status != nil && !options.Status.Valid() {
		return &domain.InvalidValueError{Field: "status", Value: options.Status.String()}
	}
	if options.Limit != nil && *options.Limit <= 0 {
		return &domain.InvalidValueError{Field: "limit", Value: strconv.Itoa(*options.Limit)}
	}
	return nil
}

func listQuery(options ListOptions) (string, []any) {
	clauses := make([]string, 0, 3)
	arguments := make([]any, 0, 4)
	if options.Type != nil {
		clauses = append(clauses, "type = ?")
		arguments = append(arguments, options.Type.String())
	}
	if options.Project != nil {
		clauses = append(clauses, "project = ?")
		arguments = append(arguments, *options.Project)
	}
	if options.Status != nil {
		clauses = append(clauses, "status = ?")
		arguments = append(arguments, options.Status.String())
	}

	query := `SELECT ` + recordColumns + ` FROM records`
	if len(clauses) != 0 {
		query += ` WHERE ` + strings.Join(clauses, ` AND `)
	}
	query += ` ORDER BY created_at DESC, id DESC`
	if options.Limit != nil {
		query += ` LIMIT ?`
		arguments = append(arguments, *options.Limit)
	}
	return query, arguments
}
