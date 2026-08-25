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

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}
