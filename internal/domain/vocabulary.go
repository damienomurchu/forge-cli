// Package domain defines Forge's product model independently of CLI, storage,
// and presentation concerns.
package domain

import "fmt"

// InvalidValueError reports a value outside an approved domain vocabulary.
type InvalidValueError struct {
	Field string
	Value string
}

func (e *InvalidValueError) Error() string {
	return fmt.Sprintf("invalid %s %q", e.Field, e.Value)
}

// RecordType identifies the kind of a Forge record.
type RecordType string

const (
	RecordTypeCapture  RecordType = "capture"
	RecordTypeFriction RecordType = "friction"
)

func (v RecordType) String() string { return string(v) }

// Valid reports whether v is an approved record type.
func (v RecordType) Valid() bool {
	return v == RecordTypeCapture || v == RecordTypeFriction
}

// ParseRecordType parses an exact, normalized record type.
func ParseRecordType(value string) (RecordType, error) {
	return parseValue("record type", value, RecordType(value))
}

// Status is a record's lifecycle state.
type Status string

const (
	StatusCaptured  Status = "captured"
	StatusReviewing Status = "reviewing"
	StatusCandidate Status = "candidate"
	StatusAutomated Status = "automated"
	StatusDismissed Status = "dismissed"
)

func (v Status) String() string { return string(v) }

// Valid reports whether v is an approved status.
func (v Status) Valid() bool {
	switch v {
	case StatusCaptured, StatusReviewing, StatusCandidate, StatusAutomated, StatusDismissed:
		return true
	default:
		return false
	}
}

// ParseStatus parses an exact, normalized lifecycle status.
func ParseStatus(value string) (Status, error) {
	return parseValue("status", value, Status(value))
}

// CaptureKind classifies a capture.
type CaptureKind string

const (
	CaptureKindThought     CaptureKind = "thought"
	CaptureKindIdea        CaptureKind = "idea"
	CaptureKindObservation CaptureKind = "observation"
	CaptureKindQuestion    CaptureKind = "question"
	CaptureKindDecision    CaptureKind = "decision"
	CaptureKindSeed        CaptureKind = "seed"
)

func (v CaptureKind) String() string { return string(v) }

// Valid reports whether v is an approved capture kind.
func (v CaptureKind) Valid() bool {
	switch v {
	case CaptureKindThought, CaptureKindIdea, CaptureKindObservation,
		CaptureKindQuestion, CaptureKindDecision, CaptureKindSeed:
		return true
	default:
		return false
	}
}

// ParseCaptureKind parses an exact, normalized capture kind.
func ParseCaptureKind(value string) (CaptureKind, error) {
	return parseValue("capture kind", value, CaptureKind(value))
}

// Frequency describes how often friction occurs.
type Frequency string

const (
	FrequencyDaily      Frequency = "daily"
	FrequencyWeekly     Frequency = "weekly"
	FrequencyMonthly    Frequency = "monthly"
	FrequencyOccasional Frequency = "occasional"
	FrequencyUnknown    Frequency = "unknown"
)

func (v Frequency) String() string { return string(v) }

// Valid reports whether v is an approved friction frequency.
func (v Frequency) Valid() bool {
	switch v {
	case FrequencyDaily, FrequencyWeekly, FrequencyMonthly, FrequencyOccasional, FrequencyUnknown:
		return true
	default:
		return false
	}
}

// ParseFrequency parses an exact, normalized friction frequency.
func ParseFrequency(value string) (Frequency, error) {
	return parseValue("frequency", value, Frequency(value))
}

// Impact describes the severity of friction.
type Impact string

const (
	ImpactLow     Impact = "low"
	ImpactMedium  Impact = "medium"
	ImpactHigh    Impact = "high"
	ImpactUnknown Impact = "unknown"
)

func (v Impact) String() string { return string(v) }

// Valid reports whether v is an approved friction impact.
func (v Impact) Valid() bool {
	switch v {
	case ImpactLow, ImpactMedium, ImpactHigh, ImpactUnknown:
		return true
	default:
		return false
	}
}

// ParseImpact parses an exact, normalized friction impact.
func ParseImpact(value string) (Impact, error) {
	return parseValue("impact", value, Impact(value))
}

// Category classifies the source of friction.
type Category string

const (
	CategoryInformationFinding Category = "information-finding"
	CategoryRepeatedAction     Category = "repeated-action"
	CategoryContextSwitching   Category = "context-switching"
	CategoryRemembering        Category = "remembering"
	CategoryVerification       Category = "verification"
	CategoryWaiting            Category = "waiting"
	CategoryOther              Category = "other"
)

func (v Category) String() string { return string(v) }

// Valid reports whether v is an approved friction category.
func (v Category) Valid() bool {
	switch v {
	case CategoryInformationFinding, CategoryRepeatedAction, CategoryContextSwitching,
		CategoryRemembering, CategoryVerification, CategoryWaiting, CategoryOther:
		return true
	default:
		return false
	}
}

// ParseCategory parses an exact, normalized friction category.
func ParseCategory(value string) (Category, error) {
	return parseValue("category", value, Category(value))
}

type validString interface {
	~string
	Valid() bool
}

func parseValue[T validString](field, original string, value T) (T, error) {
	if !value.Valid() {
		var zero T
		return zero, &InvalidValueError{Field: field, Value: original}
	}
	return value, nil
}
