// Package domain defines Forge's product model independently of CLI, storage,
// and presentation concerns.
package domain

import "fmt"

// InvalidValueError reports a value that violates a domain rule.
type InvalidValueError struct {
	Field string
	Value string
}

func (e *InvalidValueError) Error() string {
	return fmt.Sprintf("invalid %s %q", e.Field, e.Value)
}

// CaptureType identifies the workflow-specific type of a capture.
type CaptureType string

const (
	CaptureTypeFriction CaptureType = "friction"
	CaptureTypeAction   CaptureType = "action"
	CaptureTypeFollowUp CaptureType = "follow-up"
	CaptureTypeDecision CaptureType = "decision"
)

func (v CaptureType) String() string { return string(v) }

// Valid reports whether v is an approved capture type.
func (v CaptureType) Valid() bool {
	switch v {
	case CaptureTypeFriction, CaptureTypeAction, CaptureTypeFollowUp, CaptureTypeDecision:
		return true
	default:
		return false
	}
}

// ParseCaptureType parses an exact, normalized capture type.
func ParseCaptureType(value string) (CaptureType, error) {
	return parseValue("capture type", value, CaptureType(value))
}

// CaptureTypes returns the approved capture types in interactive display order.
func CaptureTypes() []CaptureType {
	return []CaptureType{
		CaptureTypeFriction,
		CaptureTypeAction,
		CaptureTypeFollowUp,
		CaptureTypeDecision,
	}
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
