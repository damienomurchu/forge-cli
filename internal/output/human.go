package output

import (
	"io"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// WriteCreated writes the concise human confirmation for a newly created
// record. Validation completes before any output is written.
func WriteCreated(w io.Writer, record domain.Record) error {
	if err := record.Validate(); err != nil {
		return err
	}
	_, err := io.WriteString(w, "Created "+record.Type.String()+" "+record.ID.String()+"\n")
	return err
}

// WriteUpdated writes the concise human confirmation for a record's resulting
// status. Validation completes before any output is written.
func WriteUpdated(w io.Writer, record domain.Record) error {
	if err := record.Validate(); err != nil {
		return err
	}
	_, err := io.WriteString(w, "Updated "+record.ID.String()+" to "+record.Status.String()+"\n")
	return err
}
