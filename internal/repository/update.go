package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// UpdateStatus changes only a record's status and updated timestamp. It returns
// the complete resulting record and whether a database update occurred.
func (r *Repository) UpdateStatus(
	ctx context.Context,
	id domain.ID,
	status domain.Status,
	now time.Time,
) (domain.Record, bool, error) {
	if !status.Valid() {
		return domain.Record{}, false, &domain.InvalidValueError{
			Field: "status",
			Value: status.String(),
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Record{}, false, fmt.Errorf("begin status update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stored, err := scanStoredRecord(tx.QueryRowContext(ctx,
		`SELECT `+recordColumns+` FROM records WHERE id = ?`,
		id.String(),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Record{}, false, ErrRecordNotFound
	}
	if err != nil {
		return domain.Record{}, false, fmt.Errorf("find record for status update: %w", err)
	}
	record, err := decodeStoredRecord(ctx, tx, stored)
	if err != nil {
		return domain.Record{}, false, fmt.Errorf("decode record for status update: %w", err)
	}
	updated, err := record.WithStatus(status, now)
	if err != nil {
		return domain.Record{}, false, fmt.Errorf("validate status update: %w", err)
	}
	if updated.Status == record.Status {
		return record, false, nil
	}

	result, err := tx.ExecContext(ctx,
		`UPDATE records SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
		updated.Status.String(),
		updated.UpdatedAt.String(),
		updated.ID.String(),
		record.Status.String(),
	)
	if err != nil {
		return domain.Record{}, false, fmt.Errorf("update record status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return domain.Record{}, false, fmt.Errorf("inspect status update: %w", err)
	}
	if rowsAffected != 1 {
		return domain.Record{}, false, fmt.Errorf("record changed during status update")
	}
	if err := tx.Commit(); err != nil {
		return domain.Record{}, false, fmt.Errorf("commit status update: %w", err)
	}
	return updated, true, nil
}
