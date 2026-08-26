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

// unifiedCaptureLister is the repository boundary required to execute a
// unified list request. repository.Repository satisfies it directly.
type unifiedCaptureLister interface {
	ListUnifiedCaptures(context.Context, repository.UnifiedCaptureFilters) ([]domain.Capture, error)
}

// executeUnifiedList queries and renders a parsed unified list request.
// Command dispatch remains responsible for storage discovery and session
// ownership.
func executeUnifiedList(
	ctx context.Context,
	request unifiedListRequest,
	lister unifiedCaptureLister,
	stdout io.Writer,
) error {
	if lister == nil {
		return fmt.Errorf("list unified captures: repository is required")
	}
	if stdout == nil {
		return fmt.Errorf("write list result: writer is required")
	}
	captures, err := lister.ListUnifiedCaptures(ctx, request.filters)
	if err != nil {
		return fmt.Errorf("list unified captures: %w", err)
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
