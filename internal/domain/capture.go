package domain

import (
	"io"
	"time"
)

// CaptureInput contains finalized capture values before domain normalization.
// Command-layer defaults and prompting are resolved before construction.
type CaptureInput struct {
	Description string
	Project     string
	Kind        CaptureKind
	Tags        string
}

// NewCapture constructs a canonical capture record. All input is validated
// before randomness is consumed.
func NewCapture(input CaptureInput, now time.Time, random io.Reader) (Record, error) {
	description, err := NormalizeDescription(input.Description)
	if err != nil {
		return Record{}, err
	}
	if !input.Kind.Valid() {
		return Record{}, &InvalidValueError{Field: "capture kind", Value: input.Kind.String()}
	}

	project := NormalizeOptionalText(input.Project)
	tags := NormalizeTags(input.Tags)
	id, err := GenerateID(RecordTypeCapture, random)
	if err != nil {
		return Record{}, err
	}
	timestamp := NewTimestamp(now)

	return Record{
		ID:          id,
		Type:        RecordTypeCapture,
		Description: description,
		Project:     project,
		Status:      StatusCaptured,
		Details: RecordDetails{Capture: &CaptureDetails{
			Kind: input.Kind,
			Tags: tags,
		}},
		CreatedAt: timestamp,
		UpdatedAt: timestamp,
	}, nil
}
