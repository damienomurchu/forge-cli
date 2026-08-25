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

// NormalizeTags parses comma-separated tags, trims and lowercases each tag,
// removes empty values and duplicates, and preserves first-seen order.
func NormalizeTags(value string) []string {
	tags := make([]string, 0)
	seen := make(map[string]struct{})
	for tag := range strings.SplitSeq(value, ",") {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	return tags
}
