package domain

import (
	"slices"
	"strings"
)

// Record is Forge's shared domain representation for captures and friction.
type Record struct {
	ID          ID
	Type        RecordType
	Description string
	Project     *string
	Status      Status
	Details     RecordDetails
	CreatedAt   Timestamp
	UpdatedAt   Timestamp
}

// RecordDetails holds exactly one type-specific details shape.
type RecordDetails struct {
	Capture  *CaptureDetails
	Friction *FrictionDetails
}

// CaptureDetails contains fields belonging only to capture records.
type CaptureDetails struct {
	Kind CaptureKind
	Tags []string
}

// FrictionDetails contains fields belonging only to friction records.
type FrictionDetails struct {
	Frequency         Frequency
	Impact            Impact
	Category          Category
	CurrentWorkaround *string
}

// Validate checks that a record satisfies the canonical domain model.
func (r Record) Validate() error {
	if !r.Type.Valid() {
		return &InvalidValueError{Field: "record type", Value: r.Type.String()}
	}
	if err := ValidateID(r.ID, r.Type); err != nil {
		return err
	}
	normalizedDescription, err := NormalizeDescription(r.Description)
	if err != nil {
		return err
	}
	if normalizedDescription != r.Description {
		return &InvalidValueError{Field: "description", Value: r.Description}
	}
	if !optionalTextIsNormalized(r.Project) {
		return &InvalidValueError{Field: "project", Value: *r.Project}
	}
	if !r.Status.Valid() {
		return &InvalidValueError{Field: "status", Value: r.Status.String()}
	}
	if r.UpdatedAt.Time().Before(r.CreatedAt.Time()) {
		return &InvalidValueError{Field: "updated_at", Value: r.UpdatedAt.String()}
	}

	if r.Type == RecordTypeCapture {
		if r.Details.Capture == nil || r.Details.Friction != nil {
			return &InvalidValueError{Field: "details", Value: r.Type.String()}
		}
		return r.Details.Capture.validate()
	}
	if r.Details.Friction == nil || r.Details.Capture != nil {
		return &InvalidValueError{Field: "details", Value: r.Type.String()}
	}
	return r.Details.Friction.validate()
}

func (d CaptureDetails) validate() error {
	if !d.Kind.Valid() {
		return &InvalidValueError{Field: "capture kind", Value: d.Kind.String()}
	}
	if d.Tags == nil {
		return &InvalidValueError{Field: "tags", Value: "null"}
	}
	normalized := NormalizeTags(strings.Join(d.Tags, ","))
	if !slices.Equal(normalized, d.Tags) {
		return &InvalidValueError{Field: "tags", Value: strings.Join(d.Tags, ",")}
	}
	return nil
}

func (d FrictionDetails) validate() error {
	if !d.Frequency.Valid() {
		return &InvalidValueError{Field: "frequency", Value: d.Frequency.String()}
	}
	if !d.Impact.Valid() {
		return &InvalidValueError{Field: "impact", Value: d.Impact.String()}
	}
	if !d.Category.Valid() {
		return &InvalidValueError{Field: "category", Value: d.Category.String()}
	}
	if !optionalTextIsNormalized(d.CurrentWorkaround) {
		return &InvalidValueError{
			Field: "current workaround",
			Value: *d.CurrentWorkaround,
		}
	}
	return nil
}

func optionalTextIsNormalized(value *string) bool {
	if value == nil {
		return true
	}
	normalized := NormalizeOptionalText(*value)
	return normalized != nil && *normalized == *value
}
