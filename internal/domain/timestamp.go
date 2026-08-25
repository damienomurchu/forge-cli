package domain

import "time"

const timestampLayout = "2006-01-02T15:04:05.000000Z"

// Timestamp is a UTC instant with microsecond precision.
type Timestamp struct {
	value time.Time
}

// NewTimestamp normalizes an instant for Forge's timestamp representation.
func NewTimestamp(value time.Time) Timestamp {
	normalized := value.Round(0).UTC().Truncate(time.Microsecond)
	return Timestamp{value: normalized}
}

// ParseTimestamp parses Forge's exact timestamp representation.
func ParseTimestamp(value string) (Timestamp, error) {
	parsed, err := time.Parse(timestampLayout, value)
	if err != nil {
		return Timestamp{}, &InvalidValueError{Field: "timestamp", Value: value}
	}
	return NewTimestamp(parsed), nil
}

// String returns the canonical UTC representation with six fractional digits.
func (t Timestamp) String() string {
	return t.value.Format(timestampLayout)
}

// Time returns the normalized standard-library time value.
func (t Timestamp) Time() time.Time {
	return t.value
}
