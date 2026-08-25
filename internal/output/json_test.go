package output

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestWriteRecordJSONMatchesGolden(t *testing.T) {
	tests := []struct {
		name   string
		record domain.Record
	}{
		{name: "capture", record: captureRecord(t)},
		{name: "friction", record: frictionRecord(t)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", tt.name+".golden"))
			if err != nil {
				t.Fatal(err)
			}

			var got bytes.Buffer
			if err := WriteRecordJSON(&got, tt.record); err != nil {
				t.Fatalf("WriteRecordJSON() error = %v", err)
			}
			if got.String() != string(want) {
				t.Errorf("JSON mismatch\ngot:  %s\nwant: %s", got.Bytes(), want)
			}
		})
	}
}

func TestWriteRecordJSONEmitsNullAndEmptyArray(t *testing.T) {
	record, err := domain.NewCapture(domain.CaptureInput{
		Description: "An unclassified thought",
		Kind:        domain.CaptureKindThought,
	}, time.Date(2026, time.August, 25, 9, 14, 3, 0, time.UTC), bytes.NewReader(make([]byte, 16)))
	if err != nil {
		t.Fatal(err)
	}

	var got bytes.Buffer
	if err := WriteRecordJSON(&got, record); err != nil {
		t.Fatalf("WriteRecordJSON() error = %v", err)
	}
	want := "{\"id\":\"cap_00000000000000000000000000000000\",\"type\":\"capture\",\"description\":\"An unclassified thought\",\"project\":null,\"status\":\"captured\",\"details\":{\"kind\":\"thought\",\"tags\":[]},\"created_at\":\"2026-08-25T09:14:03.000000Z\",\"updated_at\":\"2026-08-25T09:14:03.000000Z\"}\n"
	if got.String() != want {
		t.Errorf("WriteRecordJSON() = %q, want %q", got.String(), want)
	}
}

func TestWriteRecordJSONValidatesBeforeWriting(t *testing.T) {
	record := captureRecord(t)
	record.Description = " invalid "

	var got bytes.Buffer
	err := WriteRecordJSON(&got, record)
	var invalid *domain.InvalidValueError
	if !errors.As(err, &invalid) {
		t.Fatalf("WriteRecordJSON() error = %T %v, want *domain.InvalidValueError", err, err)
	}
	if got.Len() != 0 {
		t.Errorf("WriteRecordJSON() wrote %q before validation failed", got.String())
	}
}

func TestWriteRecordsJSONMatchesGoldenAndPreservesOrder(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "records.golden"))
	if err != nil {
		t.Fatal(err)
	}
	records := []domain.Record{captureRecord(t), frictionRecord(t)}

	var got bytes.Buffer
	if err := WriteRecordsJSON(&got, records); err != nil {
		t.Fatalf("WriteRecordsJSON() error = %v", err)
	}
	if got.String() != string(want) {
		t.Errorf("JSON mismatch\ngot:  %s\nwant: %s", got.Bytes(), want)
	}
}

func TestWriteRecordsJSONEmitsEmptyArray(t *testing.T) {
	for _, records := range [][]domain.Record{nil, {}} {
		var got bytes.Buffer
		if err := WriteRecordsJSON(&got, records); err != nil {
			t.Fatalf("WriteRecordsJSON() error = %v", err)
		}
		if got.String() != "[]\n" {
			t.Errorf("WriteRecordsJSON() = %q, want %q", got.String(), "[]\n")
		}
	}
}

func TestWriteRecordsJSONValidatesAllRecordsBeforeWriting(t *testing.T) {
	invalidRecord := frictionRecord(t)
	invalidRecord.Details.Friction.Impact = "invalid"
	records := []domain.Record{captureRecord(t), invalidRecord}

	var got bytes.Buffer
	err := WriteRecordsJSON(&got, records)
	var invalid *domain.InvalidValueError
	if !errors.As(err, &invalid) {
		t.Fatalf("WriteRecordsJSON() error = %T %v, want *domain.InvalidValueError", err, err)
	}
	if got.Len() != 0 {
		t.Errorf("WriteRecordsJSON() wrote %q before validation failed", got.String())
	}
}

func captureRecord(t *testing.T) domain.Record {
	t.Helper()
	random := mustHex(t, "7c6b1d85d8ec46e4a4f975e182bf8109")
	record, err := domain.NewCapture(domain.CaptureInput{
		Description: "Measure command startup time",
		Project:     "forge",
		Kind:        domain.CaptureKindObservation,
		Tags:        "performance,cli",
	}, time.Date(2026, time.August, 25, 9, 14, 3, 123456000, time.UTC), bytes.NewReader(random))
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func frictionRecord(t *testing.T) domain.Record {
	t.Helper()
	random := mustHex(t, "f2308c1797cf4e77ac076e6af5ff1616")
	record, err := domain.NewFriction(domain.FrictionInput{
		Description: "Releases require repeated manual checks",
		Frequency:   domain.FrequencyMonthly,
		Impact:      domain.ImpactHigh,
		Category:    domain.CategoryVerification,
	}, time.Date(2026, time.August, 25, 9, 18, 41, 654321000, time.UTC), bytes.NewReader(random))
	if err != nil {
		t.Fatal(err)
	}
	record.Status = domain.StatusReviewing
	record.UpdatedAt = mustTimestamp(t, "2026-08-26T11:02:19.000000Z")
	return record
}

func mustHex(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func mustTimestamp(t *testing.T, value string) domain.Timestamp {
	t.Helper()
	timestamp, err := domain.ParseTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}
