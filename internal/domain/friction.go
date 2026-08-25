package domain

import (
	"io"
	"time"
)

// FrictionInput contains finalized friction values before domain normalization.
// Command-layer defaults and prompting are resolved before construction.
type FrictionInput struct {
	Description       string
	Project           string
	Frequency         Frequency
	Impact            Impact
	Category          Category
	CurrentWorkaround string
}

// NewFriction constructs a canonical friction record. All input is validated
// before randomness is consumed.
func NewFriction(input FrictionInput, now time.Time, random io.Reader) (Record, error) {
	description, err := NormalizeDescription(input.Description)
	if err != nil {
		return Record{}, err
	}
	if !input.Frequency.Valid() {
		return Record{}, &InvalidValueError{Field: "frequency", Value: input.Frequency.String()}
	}
	if !input.Impact.Valid() {
		return Record{}, &InvalidValueError{Field: "impact", Value: input.Impact.String()}
	}
	if !input.Category.Valid() {
		return Record{}, &InvalidValueError{Field: "category", Value: input.Category.String()}
	}

	project := NormalizeOptionalText(input.Project)
	workaround := NormalizeOptionalText(input.CurrentWorkaround)
	id, err := GenerateID(RecordTypeFriction, random)
	if err != nil {
		return Record{}, err
	}
	timestamp := NewTimestamp(now)

	return Record{
		ID:          id,
		Type:        RecordTypeFriction,
		Description: description,
		Project:     project,
		Status:      StatusCaptured,
		Details: RecordDetails{Friction: &FrictionDetails{
			Frequency:         input.Frequency,
			Impact:            input.Impact,
			Category:          input.Category,
			CurrentWorkaround: workaround,
		}},
		CreatedAt: timestamp,
		UpdatedAt: timestamp,
	}, nil
}
