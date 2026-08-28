package repository

import (
	"context"
	"fmt"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// DeleteByID permanently deletes one capture.
func (r *Repository) DeleteByID(ctx context.Context, id domain.ID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM records WHERE id = ?`, id.String())
	if err != nil {
		return fmt.Errorf("delete capture: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted capture count: %w", err)
	}
	if deleted == 0 {
		return ErrRecordNotFound
	}
	if deleted != 1 {
		return fmt.Errorf("deleted %d captures, want 1", deleted)
	}
	return nil
}
