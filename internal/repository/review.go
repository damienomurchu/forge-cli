package repository

import (
	"context"
	"fmt"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// Review returns actionable friction newest-first without modifying the
// database.
func (r *Repository) Review(ctx context.Context) ([]domain.Record, error) {
	records, err := r.readRecords(ctx,
		`SELECT `+recordColumns+` FROM records
		 WHERE type = ? AND status IN (?, ?, ?)
		 ORDER BY created_at DESC, id DESC`,
		domain.RecordTypeFriction.String(),
		domain.StatusCaptured.String(),
		domain.StatusReviewing.String(),
		domain.StatusCandidate.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("review friction: %w", err)
	}
	return records, nil
}
