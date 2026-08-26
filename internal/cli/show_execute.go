package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/output"
)

// captureFinder is the repository boundary required to execute a
// show request. repository.Repository satisfies it directly.
type captureFinder interface {
	FindByID(context.Context, domain.ID) (domain.Capture, error)
}

// executeShow finds and renders one capture. Command dispatch
// remains responsible for storage discovery, session ownership, and presenting
// a not-found error to the user.
func executeShow(
	ctx context.Context,
	id domain.ID,
	jsonOutput bool,
	finder captureFinder,
	stdout io.Writer,
) error {
	if finder == nil {
		return fmt.Errorf("find capture: repository is required")
	}
	if stdout == nil {
		return fmt.Errorf("write show result: writer is required")
	}
	capture, err := finder.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find capture: %w", err)
	}

	var rendered bytes.Buffer
	if jsonOutput {
		err = output.WriteCaptureJSON(&rendered, capture)
	} else {
		err = output.WriteCapture(&rendered, capture)
	}
	if err != nil {
		return fmt.Errorf("render show result: %w", err)
	}
	if _, err := io.Copy(stdout, &rendered); err != nil {
		return fmt.Errorf("write show result: %w", err)
	}
	return nil
}
