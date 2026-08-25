// Package output renders validated Forge domain values for users and machines.
package output

import (
	"encoding/json"
	"io"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// WriteRecordJSON writes one validated record as a JSON object followed by a
// newline. Validation completes before any output is written.
func WriteRecordJSON(w io.Writer, record domain.Record) error {
	if err := record.Validate(); err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(recordToJSON(record))
}

// WriteRecordsJSON writes validated records as one JSON array followed by a
// newline. Every record is validated before any output is written.
func WriteRecordsJSON(w io.Writer, records []domain.Record) error {
	encoded := make([]recordJSON, len(records))
	for i, record := range records {
		if err := record.Validate(); err != nil {
			return err
		}
		encoded[i] = recordToJSON(record)
	}
	return json.NewEncoder(w).Encode(encoded)
}

func recordToJSON(record domain.Record) recordJSON {
	var details any
	if record.Type == domain.RecordTypeCapture {
		details = captureDetailsJSON{
			Kind: record.Details.Capture.Kind.String(),
			Tags: record.Details.Capture.Tags,
		}
	} else {
		details = frictionDetailsJSON{
			Frequency:         record.Details.Friction.Frequency.String(),
			Impact:            record.Details.Friction.Impact.String(),
			Category:          record.Details.Friction.Category.String(),
			CurrentWorkaround: record.Details.Friction.CurrentWorkaround,
		}
	}

	return recordJSON{
		ID:          record.ID.String(),
		Type:        record.Type.String(),
		Description: record.Description,
		Project:     record.Project,
		Status:      record.Status.String(),
		Details:     details,
		CreatedAt:   record.CreatedAt.String(),
		UpdatedAt:   record.UpdatedAt.String(),
	}
}

type recordJSON struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Project     *string `json:"project"`
	Status      string  `json:"status"`
	Details     any     `json:"details"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type captureDetailsJSON struct {
	Kind string   `json:"kind"`
	Tags []string `json:"tags"`
}

type frictionDetailsJSON struct {
	Frequency         string  `json:"frequency"`
	Impact            string  `json:"impact"`
	Category          string  `json:"category"`
	CurrentWorkaround *string `json:"current_workaround"`
}
