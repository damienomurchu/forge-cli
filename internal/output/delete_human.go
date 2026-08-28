package output

import (
	"fmt"
	"io"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// WriteCaptureDeleted writes the human success result for a deleted capture.
func WriteCaptureDeleted(w io.Writer, id domain.ID) error {
	_, err := fmt.Fprintf(w, "Deleted capture %s\n", EscapeText(id.String()))
	return err
}
