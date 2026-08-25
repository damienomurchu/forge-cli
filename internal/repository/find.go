package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// ErrRecordNotFound identifies a lookup for an ID that is not stored.
var ErrRecordNotFound = errors.New("record not found")

const recordColumns = `
	id, type, description, project, status,
	capture_kind, friction_frequency, friction_impact,
	friction_category, current_workaround, created_at, updated_at`

type rowScanner interface {
	Scan(dest ...any) error
}

type recordQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type storedRecord struct {
	id                string
	recordType        string
	description       string
	project           sql.NullString
	status            string
	captureKind       sql.NullString
	frictionFrequency sql.NullString
	frictionImpact    sql.NullString
	frictionCategory  sql.NullString
	currentWorkaround sql.NullString
	createdAt         string
	updatedAt         string
}

// FindByID returns the complete record with id without modifying the database.
func (r *Repository) FindByID(ctx context.Context, id domain.ID) (domain.Record, error) {
	stored, err := scanStoredRecord(r.db.QueryRowContext(ctx,
		`SELECT `+recordColumns+` FROM records WHERE id = ?`,
		id.String(),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Record{}, ErrRecordNotFound
	}
	if err != nil {
		return domain.Record{}, fmt.Errorf("find record: %w", err)
	}

	record, err := r.decodeStoredRecord(ctx, stored)
	if err != nil {
		return domain.Record{}, fmt.Errorf("decode stored record: %w", err)
	}
	return record, nil
}

func scanStoredRecord(scanner rowScanner) (storedRecord, error) {
	var stored storedRecord
	err := scanner.Scan(
		&stored.id,
		&stored.recordType,
		&stored.description,
		&stored.project,
		&stored.status,
		&stored.captureKind,
		&stored.frictionFrequency,
		&stored.frictionImpact,
		&stored.frictionCategory,
		&stored.currentWorkaround,
		&stored.createdAt,
		&stored.updatedAt,
	)
	return stored, err
}

func (r *Repository) decodeStoredRecord(ctx context.Context, stored storedRecord) (domain.Record, error) {
	return decodeStoredRecord(ctx, r.db, stored)
}

func decodeStoredRecord(ctx context.Context, queryer recordQueryer, stored storedRecord) (domain.Record, error) {
	recordType, err := domain.ParseRecordType(stored.recordType)
	if err != nil {
		return domain.Record{}, err
	}
	status, err := domain.ParseStatus(stored.status)
	if err != nil {
		return domain.Record{}, err
	}
	createdAt, err := domain.ParseTimestamp(stored.createdAt)
	if err != nil {
		return domain.Record{}, err
	}
	updatedAt, err := domain.ParseTimestamp(stored.updatedAt)
	if err != nil {
		return domain.Record{}, err
	}

	record := domain.Record{
		ID:          domain.ID(stored.id),
		Type:        recordType,
		Description: stored.description,
		Project:     stringPointerFromNull(stored.project),
		Status:      status,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	switch recordType {
	case domain.RecordTypeCapture:
		if !stored.captureKind.Valid || stored.frictionFrequency.Valid ||
			stored.frictionImpact.Valid || stored.frictionCategory.Valid ||
			stored.currentWorkaround.Valid {
			return domain.Record{}, fmt.Errorf("invalid capture column shape")
		}
		kind, err := domain.ParseCaptureKind(stored.captureKind.String)
		if err != nil {
			return domain.Record{}, err
		}
		tags, err := loadCaptureTags(ctx, queryer, record.ID)
		if err != nil {
			return domain.Record{}, err
		}
		record.Details.Capture = &domain.CaptureDetails{Kind: kind, Tags: tags}
	case domain.RecordTypeFriction:
		if stored.captureKind.Valid || !stored.frictionFrequency.Valid ||
			!stored.frictionImpact.Valid || !stored.frictionCategory.Valid {
			return domain.Record{}, fmt.Errorf("invalid friction column shape")
		}
		frequency, err := domain.ParseFrequency(stored.frictionFrequency.String)
		if err != nil {
			return domain.Record{}, err
		}
		impact, err := domain.ParseImpact(stored.frictionImpact.String)
		if err != nil {
			return domain.Record{}, err
		}
		category, err := domain.ParseCategory(stored.frictionCategory.String)
		if err != nil {
			return domain.Record{}, err
		}
		record.Details.Friction = &domain.FrictionDetails{
			Frequency:         frequency,
			Impact:            impact,
			Category:          category,
			CurrentWorkaround: stringPointerFromNull(stored.currentWorkaround),
		}
	}

	if err := record.Validate(); err != nil {
		return domain.Record{}, err
	}
	return record, nil
}

func loadCaptureTags(ctx context.Context, queryer recordQueryer, id domain.ID) ([]string, error) {
	rows, err := queryer.QueryContext(ctx,
		`SELECT position, tag FROM record_tags WHERE record_id = ? ORDER BY position ASC`,
		id.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("load capture tags: %w", err)
	}
	defer rows.Close()

	tags := make([]string, 0)
	for rows.Next() {
		var position int
		var tag string
		if err := rows.Scan(&position, &tag); err != nil {
			return nil, fmt.Errorf("scan capture tag: %w", err)
		}
		if position != len(tags) {
			return nil, fmt.Errorf("capture tag position %d is not contiguous", position)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate capture tags: %w", err)
	}
	return tags, nil
}

func stringPointerFromNull(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
