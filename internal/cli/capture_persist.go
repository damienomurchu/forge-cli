package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/output"
)

// captureCreator is the repository boundary required to persist a
// capture. repository.Repository satisfies it directly.
type captureCreator interface {
	CreateCapture(context.Context, domain.Capture) error
}

// persistCapture adds persistence metadata to a confirmed proposal,
// stores it, and writes the requested success representation. Command dispatch
// remains responsible for opening and closing storage.
func persistCapture(
	ctx context.Context,
	proposed domain.ProposedCapture,
	jsonOutput bool,
	now time.Time,
	random io.Reader,
	creator captureCreator,
	stdout io.Writer,
) (domain.Capture, error) {
	capture, err := domain.NewPersistedCapture(proposed, now, random)
	if err != nil {
		return domain.Capture{}, fmt.Errorf("create persisted capture: %w", err)
	}
	if creator == nil {
		return domain.Capture{}, fmt.Errorf("persist capture: repository is required")
	}
	if stdout == nil {
		return domain.Capture{}, fmt.Errorf("write capture result: writer is required")
	}
	if err := creator.CreateCapture(ctx, capture); err != nil {
		return domain.Capture{}, fmt.Errorf("persist capture: %w", err)
	}

	var rendered bytes.Buffer
	if jsonOutput {
		err = output.WriteCaptureJSON(&rendered, capture)
	} else {
		err = output.WriteCaptureCreated(&rendered, capture)
	}
	if err != nil {
		return domain.Capture{}, fmt.Errorf("render capture result: %w", err)
	}
	if _, err := io.Copy(stdout, &rendered); err != nil {
		return domain.Capture{}, fmt.Errorf("write capture result: %w", err)
	}
	return capture, nil
}
