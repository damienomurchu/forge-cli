package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

const unifiedCaptureColumns = `
	id, capture_type, description, friction_project,
	friction_frequency, friction_impact, friction_category,
	friction_current_workaround, created_at, updated_at`

type storedUnifiedCapture struct {
	id                string
	captureType       string
	description       string
	project           sql.NullString
	frequency         sql.NullString
	impact            sql.NullString
	category          sql.NullString
	currentWorkaround sql.NullString
	createdAt         string
	updatedAt         string
}

// FindUnifiedCaptureByID returns a complete capture from the staged unified
// schema without modifying the database.
func (r *Repository) FindUnifiedCaptureByID(ctx context.Context, id domain.ID) (domain.Capture, error) {
	stored, err := scanStoredUnifiedCapture(r.db.QueryRowContext(ctx,
		`SELECT `+unifiedCaptureColumns+` FROM records WHERE id = ?`,
		id.String(),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Capture{}, ErrRecordNotFound
	}
	if err != nil {
		return domain.Capture{}, fmt.Errorf("find unified capture: %w", err)
	}
	capture, err := decodeStoredUnifiedCapture(stored)
	if err != nil {
		return domain.Capture{}, fmt.Errorf("decode stored unified capture: %w", err)
	}
	return capture, nil
}

func scanStoredUnifiedCapture(scanner rowScanner) (storedUnifiedCapture, error) {
	var stored storedUnifiedCapture
	err := scanner.Scan(
		&stored.id,
		&stored.captureType,
		&stored.description,
		&stored.project,
		&stored.frequency,
		&stored.impact,
		&stored.category,
		&stored.currentWorkaround,
		&stored.createdAt,
		&stored.updatedAt,
	)
	return stored, err
}

func decodeStoredUnifiedCapture(stored storedUnifiedCapture) (domain.Capture, error) {
	captureType, err := domain.ParseCaptureType(stored.captureType)
	if err != nil {
		return domain.Capture{}, err
	}
	createdAt, err := domain.ParseTimestamp(stored.createdAt)
	if err != nil {
		return domain.Capture{}, err
	}
	updatedAt, err := domain.ParseTimestamp(stored.updatedAt)
	if err != nil {
		return domain.Capture{}, err
	}

	capture := domain.Capture{
		ID:          domain.ID(stored.id),
		Type:        captureType,
		Description: stored.description,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
	if captureType == domain.CaptureTypeFriction {
		if !stored.frequency.Valid || !stored.impact.Valid || !stored.category.Valid {
			return domain.Capture{}, fmt.Errorf("invalid friction column shape")
		}
		frequency, err := domain.ParseFrequency(stored.frequency.String)
		if err != nil {
			return domain.Capture{}, err
		}
		impact, err := domain.ParseImpact(stored.impact.String)
		if err != nil {
			return domain.Capture{}, err
		}
		category, err := domain.ParseCategory(stored.category.String)
		if err != nil {
			return domain.Capture{}, err
		}
		capture.Details.Friction = &domain.FrictionCaptureDetails{
			Project:           stringPointerFromNull(stored.project),
			Frequency:         frequency,
			Impact:            impact,
			Category:          category,
			CurrentWorkaround: stringPointerFromNull(stored.currentWorkaround),
		}
	} else {
		if stored.project.Valid || stored.frequency.Valid || stored.impact.Valid ||
			stored.category.Valid || stored.currentWorkaround.Valid {
			return domain.Capture{}, fmt.Errorf("invalid %s column shape", captureType)
		}
		switch captureType {
		case domain.CaptureTypeAction:
			capture.Details.Action = &domain.ActionCaptureDetails{}
		case domain.CaptureTypeFollowUp:
			capture.Details.FollowUp = &domain.FollowUpCaptureDetails{}
		case domain.CaptureTypeDecision:
			capture.Details.Decision = &domain.DecisionCaptureDetails{}
		}
	}
	if err := capture.Validate(); err != nil {
		return domain.Capture{}, err
	}
	return capture, nil
}
