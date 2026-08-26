package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/output"
	"github.com/damienomurchu/forge-cli/internal/repository"
)

// captureLister is the repository boundary required to execute a
// list request. repository.Repository satisfies it directly.
type captureLister interface {
	List(context.Context, repository.CaptureFilters) ([]domain.Capture, error)
}

// executeList queries and renders a parsed list request.
// Command dispatch remains responsible for storage discovery and session
// ownership.
func executeList(
	ctx context.Context,
	request listRequest,
	lister captureLister,
	stdout io.Writer,
) error {
	if lister == nil {
		return fmt.Errorf("list captures: repository is required")
	}
	if stdout == nil {
		return fmt.Errorf("write list result: writer is required")
	}
	captures, err := lister.List(ctx, request.filters)
	if err != nil {
		return fmt.Errorf("list captures: %w", err)
	}

	var rendered bytes.Buffer
	if request.json {
		err = output.WriteCapturesJSON(&rendered, captures)
	} else {
		err = output.WriteCaptureList(&rendered, captures)
	}
	if err != nil {
		return fmt.Errorf("render list result: %w", err)
	}
	if _, err := io.Copy(stdout, &rendered); err != nil {
		return fmt.Errorf("write list result: %w", err)
	}
	return nil
}
