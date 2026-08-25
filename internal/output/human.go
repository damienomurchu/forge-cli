package output

import (
	"bytes"
	"io"
	"strings"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// WriteRecord writes a complete terminal-safe human representation of one
// record. Validation completes before any output is written.
func WriteRecord(w io.Writer, record domain.Record) error {
	if err := record.Validate(); err != nil {
		return err
	}

	var rendered bytes.Buffer
	writeField := func(label, value string) {
		rendered.WriteString(label)
		rendered.WriteString(": ")
		rendered.WriteString(value)
		rendered.WriteByte('\n')
	}
	writeField("ID", EscapeText(record.ID.String()))
	writeField("Type", record.Type.String())
	writeField("Description", EscapeText(record.Description))
	writeField("Project", escapedOptionalText(record.Project))
	writeField("Status", record.Status.String())
	if record.Type == domain.RecordTypeCapture {
		writeField("Kind", record.Details.Capture.Kind.String())
		tags := make([]string, len(record.Details.Capture.Tags))
		for index, tag := range record.Details.Capture.Tags {
			tags[index] = EscapeText(tag)
		}
		if len(tags) == 0 {
			writeField("Tags", "-")
		} else {
			writeField("Tags", strings.Join(tags, ", "))
		}
	} else {
		writeField("Frequency", record.Details.Friction.Frequency.String())
		writeField("Impact", record.Details.Friction.Impact.String())
		writeField("Category", record.Details.Friction.Category.String())
		writeField("Current workaround", escapedOptionalText(record.Details.Friction.CurrentWorkaround))
	}
	writeField("Created", record.CreatedAt.String())
	writeField("Updated", record.UpdatedAt.String())
	_, err := io.Copy(w, &rendered)
	return err
}

func escapedOptionalText(value *string) string {
	if value == nil {
		return "-"
	}
	return EscapeText(*value)
}

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
