package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewTimestampNormalizesUTCAndPrecision(t *testing.T) {
	location := time.FixedZone("test", 90*60)
	input := time.Date(2026, time.August, 25, 10, 44, 3, 123456789, location)

	got := NewTimestamp(input)
	if want := "2026-08-25T09:14:03.123456Z"; got.String() != want {
		t.Errorf("NewTimestamp().String() = %q, want %q", got.String(), want)
	}
	if got.Time().Location() != time.UTC {
		t.Errorf("NewTimestamp().Time().Location() = %v, want UTC", got.Time().Location())
	}
	if got.Time().Nanosecond() != 123456000 {
		t.Errorf("NewTimestamp().Time().Nanosecond() = %d, want 123456000", got.Time().Nanosecond())
	}
}

func TestNewTimestampPadsFractionalDigits(t *testing.T) {
	input := time.Date(2026, time.August, 25, 9, 14, 3, 0, time.UTC)
	if got, want := NewTimestamp(input).String(), "2026-08-25T09:14:03.000000Z"; got != want {
		t.Errorf("NewTimestamp().String() = %q, want %q", got, want)
	}
}

func TestParseTimestampRoundTrip(t *testing.T) {
	value := "2024-02-29T23:59:59.654321Z"
	got, err := ParseTimestamp(value)
	if err != nil {
		t.Fatalf("ParseTimestamp(%q) error = %v", value, err)
	}
	if got.String() != value {
		t.Errorf("ParseTimestamp(%q).String() = %q", value, got.String())
	}
	wantTime := time.Date(2024, time.February, 29, 23, 59, 59, 654321000, time.UTC)
	if got.Time() != wantTime {
		t.Errorf("ParseTimestamp(%q).Time() = %v, want %v", value, got.Time(), wantTime)
	}
}

func TestParseTimestampRejectsNonCanonicalValues(t *testing.T) {
	values := []string{
		"",
		"2026-08-25T09:14:03Z",
		"2026-08-25T09:14:03.123Z",
		"2026-08-25T09:14:03.123456789Z",
		"2026-08-25T09:14:03.123456+00:00",
		"2026-08-25T09:14:03.123456z",
		"2026-08-25 09:14:03.123456Z",
		"2026-02-29T09:14:03.123456Z",
		" 2026-08-25T09:14:03.123456Z ",
	}

	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			got, err := ParseTimestamp(value)
			var invalid *InvalidValueError
			if !errors.As(err, &invalid) {
				t.Fatalf("ParseTimestamp(%q) error = %T %v, want *InvalidValueError", value, err, err)
			}
			if invalid.Field != "timestamp" || invalid.Value != value {
				t.Errorf("error = %#v, want invalid timestamp %q", invalid, value)
			}
			if got != (Timestamp{}) {
				t.Errorf("ParseTimestamp(%q) = %v, want zero value", value, got)
			}
		})
	}
}
