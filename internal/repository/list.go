package repository

import (
	"context"
	"fmt"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// List returns every record newest-first with ID as the deterministic
// descending tie-breaker. It does not modify the database.
func (r *Repository) List(ctx context.Context) ([]domain.Record, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+recordColumns+` FROM records ORDER BY created_at DESC, id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list records: %w", err)
	}

	storedRecords := make([]storedRecord, 0)
	for rows.Next() {
		stored, err := scanStoredRecord(rows)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan listed record: %w", err)
		}
		storedRecords = append(storedRecords, stored)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate listed records: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close listed records: %w", err)
	}

	records := make([]domain.Record, 0, len(storedRecords))
	for _, stored := range storedRecords {
		record, err := r.decodeStoredRecord(ctx, stored)
		if err != nil {
			return nil, fmt.Errorf("decode listed record: %w", err)
		}
		records = append(records, record)
	}
	return records, nil
}
