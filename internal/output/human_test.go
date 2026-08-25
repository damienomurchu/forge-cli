package output

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestWriteCreatedMatchesGolden(t *testing.T) {
	tests := []struct {
		name   string
		record domain.Record
	}{
		{name: "capture", record: captureRecord(t)},
		{name: "friction", record: frictionRecord(t)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", tt.name+"-created.golden"))
			if err != nil {
				t.Fatal(err)
			}

			var got bytes.Buffer
			if err := WriteCreated(&got, tt.record); err != nil {
				t.Fatalf("WriteCreated() error = %v", err)
			}
			if got.String() != string(want) {
				t.Errorf("WriteCreated() = %q, want %q", got.String(), want)
			}
		})
	}
}

func TestWriteRecordListMatchesGolden(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "list.golden"))
	if err != nil {
		t.Fatal(err)
	}
	records := []domain.Record{frictionRecord(t), captureRecord(t)}

	var got bytes.Buffer
	if err := WriteRecordList(&got, records); err != nil {
		t.Fatalf("WriteRecordList() error = %v", err)
	}
	if got.String() != string(want) {
		t.Errorf("WriteRecordList() = %q, want %q", got.String(), want)
	}
}

func TestWriteRecordListEscapesDescriptions(t *testing.T) {
	record := captureRecord(t)
	record.Description = "line one\nline two\t\u202e"
	var got bytes.Buffer
	if err := WriteRecordList(&got, []domain.Record{record}); err != nil {
		t.Fatalf("WriteRecordList() error = %v", err)
	}
	want := record.ID.String() + "  capture  captured  line one\\nline two\\t\\u202e\n"
	if got.String() != want {
		t.Errorf("WriteRecordList() = %q, want %q", got.String(), want)
	}
}

func TestWriteRecordListValidatesAllRecordsBeforeWriting(t *testing.T) {
	invalid := frictionRecord(t)
	invalid.Status = "invalid"
	var got bytes.Buffer
	err := WriteRecordList(&got, []domain.Record{captureRecord(t), invalid})
	var invalidValue *domain.InvalidValueError
	if !errors.As(err, &invalidValue) {
		t.Fatalf("WriteRecordList() error = %T %v, want *domain.InvalidValueError", err, err)
	}
	if got.Len() != 0 {
		t.Errorf("WriteRecordList() wrote %q before validation failed", got.String())
	}
}

func TestWriteRecordListEmptyAndWriterFailure(t *testing.T) {
	var empty bytes.Buffer
	if err := WriteRecordList(&empty, nil); err != nil {
		t.Fatalf("empty WriteRecordList() error = %v", err)
	}
	if empty.Len() != 0 {
		t.Errorf("empty WriteRecordList() = %q, want empty", empty.String())
	}

	wantErr := errors.New("writer failed")
	err := WriteRecordList(errorWriter{err: wantErr}, []domain.Record{captureRecord(t)})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WriteRecordList() error = %v, want %v", err, wantErr)
	}
}

func TestWriteCreatedValidatesBeforeWriting(t *testing.T) {
	record := captureRecord(t)
	record.ID = "invalid"

	var got bytes.Buffer
	err := WriteCreated(&got, record)
	var invalid *domain.InvalidValueError
	if !errors.As(err, &invalid) {
		t.Fatalf("WriteCreated() error = %T %v, want *domain.InvalidValueError", err, err)
	}
	if got.Len() != 0 {
		t.Errorf("WriteCreated() wrote %q before validation failed", got.String())
	}
}

func TestWriteCreatedPropagatesWriterFailure(t *testing.T) {
	wantErr := errors.New("writer failed")
	err := WriteCreated(errorWriter{err: wantErr}, captureRecord(t))
	if !errors.Is(err, wantErr) {
		t.Fatalf("WriteCreated() error = %v, want %v", err, wantErr)
	}
}

func TestWriteUpdatedMatchesGolden(t *testing.T) {
	tests := []struct {
		name   string
		record domain.Record
	}{
		{name: "capture", record: captureRecord(t)},
		{name: "friction", record: frictionRecord(t)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", tt.name+"-updated.golden"))
			if err != nil {
				t.Fatal(err)
			}

			var got bytes.Buffer
			if err := WriteUpdated(&got, tt.record); err != nil {
				t.Fatalf("WriteUpdated() error = %v", err)
			}
			if got.String() != string(want) {
				t.Errorf("WriteUpdated() = %q, want %q", got.String(), want)
			}
		})
	}
}

func TestWriteUpdatedValidatesBeforeWriting(t *testing.T) {
	record := frictionRecord(t)
	record.Status = "invalid"

	var got bytes.Buffer
	err := WriteUpdated(&got, record)
	var invalid *domain.InvalidValueError
	if !errors.As(err, &invalid) {
		t.Fatalf("WriteUpdated() error = %T %v, want *domain.InvalidValueError", err, err)
	}
	if got.Len() != 0 {
		t.Errorf("WriteUpdated() wrote %q before validation failed", got.String())
	}
}

func TestWriteUpdatedPropagatesWriterFailure(t *testing.T) {
	wantErr := errors.New("writer failed")
	err := WriteUpdated(errorWriter{err: wantErr}, frictionRecord(t))
	if !errors.Is(err, wantErr) {
		t.Fatalf("WriteUpdated() error = %v, want %v", err, wantErr)
	}
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}
