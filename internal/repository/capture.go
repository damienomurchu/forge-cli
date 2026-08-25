package repository

import (
	"context"
	"fmt"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// CreateCapture stores a validated capture and its ordered tags atomically.
func (r *Repository) CreateCapture(ctx context.Context, record domain.Record) error {
	if err := record.Validate(); err != nil {
		return fmt.Errorf("validate capture: %w", err)
	}
	if record.Type != domain.RecordTypeCapture {
		return fmt.Errorf("validate capture: record type is %q", record.Type)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin capture creation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	details := record.Details.Capture
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO records (
			id, type, description, project, status,
			capture_kind, friction_frequency, friction_impact,
			friction_category, current_workaround, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, NULL, NULL, NULL, NULL, ?, ?)`,
		record.ID.String(),
		record.Type.String(),
		record.Description,
		optionalString(record.Project),
		record.Status.String(),
		details.Kind.String(),
		record.CreatedAt.String(),
		record.UpdatedAt.String(),
	); err != nil {
		return fmt.Errorf("insert capture %s: %w", record.ID, err)
	}

	for position, tag := range details.Tags {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO record_tags(record_id, position, tag) VALUES (?, ?, ?)`,
			record.ID.String(),
			position,
			tag,
		); err != nil {
			return fmt.Errorf("insert capture %s tag %d: %w", record.ID, position, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit capture %s: %w", record.ID, err)
	}
	return nil
}

func optionalString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
