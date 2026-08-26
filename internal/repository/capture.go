package repository

import (
	"context"
	"fmt"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// CreateCapture stores a validated capture atomically.
func (r *Repository) CreateCapture(ctx context.Context, capture domain.Capture) error {
	if err := capture.Validate(); err != nil {
		return fmt.Errorf("validate capture: %w", err)
	}

	var project, frequency, impact, category, workaround any
	if capture.Type == domain.CaptureTypeFriction {
		details := capture.Details.Friction
		project = optionalString(details.Project)
		frequency = details.Frequency.String()
		impact = details.Impact.String()
		category = details.Category.String()
		workaround = optionalString(details.CurrentWorkaround)
	}

	if _, err := r.db.ExecContext(ctx, `INSERT INTO records (
		id,
		capture_type,
		description,
		friction_project,
		friction_frequency,
		friction_impact,
		friction_category,
		friction_current_workaround,
		created_at,
		updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		capture.ID.String(),
		capture.Type.String(),
		capture.Description,
		project,
		frequency,
		impact,
		category,
		workaround,
		capture.CreatedAt.String(),
		capture.UpdatedAt.String(),
	); err != nil {
		return fmt.Errorf("insert capture %s: %w", capture.ID, err)
	}
	return nil
}
