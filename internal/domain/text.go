package domain

import "strings"

// NormalizeDescription trims surrounding Unicode whitespace and rejects an
// empty result. Internal whitespace is preserved.
func NormalizeDescription(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", &InvalidValueError{Field: "description", Value: value}
	}
	return normalized, nil
}

// NormalizeOptionalText trims surrounding Unicode whitespace and returns nil
// when no text remains.
func NormalizeOptionalText(value string) *string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return nil
	}
	return &normalized
}
