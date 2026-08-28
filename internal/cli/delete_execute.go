package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/output"
)

type captureDeleter interface {
	DeleteByID(context.Context, domain.ID) error
}

func executeDelete(ctx context.Context, id domain.ID, deleter captureDeleter, stdout io.Writer) error {
	if deleter == nil {
		return fmt.Errorf("delete capture: repository is required")
	}
	if stdout == nil {
		return fmt.Errorf("write delete result: writer is required")
	}
	if err := deleter.DeleteByID(ctx, id); err != nil {
		return fmt.Errorf("delete capture: %w", err)
	}
	if err := output.WriteCaptureDeleted(stdout, id); err != nil {
		return fmt.Errorf("write delete result: %w", err)
	}
	return nil
}
