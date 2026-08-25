package repository

import (
	"context"
	"fmt"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// CreateFriction stores a validated friction record atomically.
func (r *Repository) CreateFriction(ctx context.Context, record domain.Record) error {
	if err := record.Validate(); err != nil {
		return fmt.Errorf("validate friction: %w", err)
	}
	if record.Type != domain.RecordTypeFriction {
		return fmt.Errorf("validate friction: record type is %q", record.Type)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin friction creation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	details := record.Details.Friction
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO records (
			id, type, description, project, status,
			capture_kind, friction_frequency, friction_impact,
			friction_category, current_workaround, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?)`,
		record.ID.String(),
		record.Type.String(),
		record.Description,
		optionalString(record.Project),
		record.Status.String(),
		details.Frequency.String(),
		details.Impact.String(),
		details.Category.String(),
		optionalString(details.CurrentWorkaround),
		record.CreatedAt.String(),
		record.UpdatedAt.String(),
	); err != nil {
		return fmt.Errorf("insert friction %s: %w", record.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit friction %s: %w", record.ID, err)
	}
	return nil
}
