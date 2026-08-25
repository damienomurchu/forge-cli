package output

import (
	"bytes"
	"io"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// WriteRecordList writes a compact terminal-safe line for each record in the
// provided order. Every record is validated before any output is written.
func WriteRecordList(w io.Writer, records []domain.Record) error {
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return err
		}
	}

	var rendered bytes.Buffer
	for _, record := range records {
		rendered.WriteString(record.ID.String())
		rendered.WriteString("  ")
		rendered.WriteString(record.Type.String())
		rendered.WriteString("  ")
		rendered.WriteString(record.Status.String())
		rendered.WriteString("  ")
		rendered.WriteString(EscapeText(record.Description))
		rendered.WriteByte('\n')
	}
	_, err := io.Copy(w, &rendered)
	return err
}

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
