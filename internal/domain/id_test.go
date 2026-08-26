package domain

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"
)

func TestGenerateID(t *testing.T) {
	randomBytes := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	}
	tests := []struct {
		name       string
		recordType RecordType
		want       ID
	}{
		{name: "capture", recordType: RecordTypeCapture, want: "cap_000102030405060708090a0b0c0d0e0f"},
		{name: "friction", recordType: RecordTypeFriction, want: "frc_000102030405060708090a0b0c0d0e0f"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateID(tt.recordType, bytes.NewReader(randomBytes))
			if err != nil {
				t.Fatalf("GenerateID() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("GenerateID() = %q, want %q", got, tt.want)
			}
			if got.String() != string(tt.want) {
				t.Errorf("ID.String() = %q, want %q", got.String(), tt.want)
			}
			if err := ValidateID(got, tt.recordType); err != nil {
				t.Errorf("ValidateID() error = %v", err)
			}
		})
	}
}

func TestGenerateCaptureID(t *testing.T) {
	got, err := GenerateCaptureID(bytes.NewReader([]byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	}))
	if err != nil {
		t.Fatalf("GenerateCaptureID() error = %v", err)
	}
	if want := ID("cap_000102030405060708090a0b0c0d0e0f"); got != want {
		t.Errorf("GenerateCaptureID() = %q, want %q", got, want)
	}
	for _, captureType := range CaptureTypes() {
		if err := ValidateCaptureID(got, captureType); err != nil {
			t.Errorf("ValidateCaptureID(%q, %q) error = %v", got, captureType, err)
		}
	}
}

func TestValidateCaptureIDSupportsOnlyMigratedFrictionPrefix(t *testing.T) {
	legacy := ID("frc_000102030405060708090a0b0c0d0e0f")
	if err := ValidateCaptureID(legacy, CaptureTypeFriction); err != nil {
		t.Fatalf("ValidateCaptureID() migrated friction error = %v", err)
	}
	for _, captureType := range []CaptureType{CaptureTypeAction, CaptureTypeFollowUp, CaptureTypeDecision} {
		err := ValidateCaptureID(legacy, captureType)
		var invalid *InvalidValueError
		if !errors.As(err, &invalid) || invalid.Field != "capture ID" {
			t.Errorf("ValidateCaptureID(%q, %q) error = %T %v, want invalid capture ID",
				legacy, captureType, err, err)
		}
	}
}

func TestValidateCaptureIDRejectsMalformedValues(t *testing.T) {
	values := []ID{
		"",
		"cap_000102030405060708090a0b0c0d0e",
		"cap_000102030405060708090a0b0c0d0e0f0",
		"cap_000102030405060708090A0B0C0D0E0F",
		"cap_000102030405060708090a0b0c0d0e0g",
	}
	for _, value := range values {
		t.Run(value.String(), func(t *testing.T) {
			err := ValidateCaptureID(value, CaptureTypeAction)
			var invalid *InvalidValueError
			if !errors.As(err, &invalid) || invalid.Field != "capture ID" || invalid.Value != value.String() {
				t.Fatalf("ValidateCaptureID(%q) error = %#v, want invalid capture ID", value, invalid)
			}
		})
	}
}

func TestGenerateIDRejectsInvalidRecordTypeBeforeReading(t *testing.T) {
	got, err := GenerateID(RecordType("invalid"), panicReader{})
	var invalid *InvalidValueError
	if !errors.As(err, &invalid) {
		t.Fatalf("GenerateID() error = %T %v, want *InvalidValueError", err, err)
	}
	if invalid.Field != "record type" || invalid.Value != "invalid" {
		t.Errorf("error = %#v, want invalid record type", invalid)
	}
	if got != "" {
		t.Errorf("GenerateID() = %q, want empty", got)
	}
}

func TestGenerateIDRequiresExactly128Bits(t *testing.T) {
	got, err := GenerateID(RecordTypeCapture, bytes.NewReader(make([]byte, randomIDBytes-1)))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("GenerateID() error = %v, want io.ErrUnexpectedEOF", err)
	}
	if got != "" {
		t.Errorf("GenerateID() = %q, want empty", got)
	}

	got, err = GenerateCaptureID(bytes.NewReader(make([]byte, randomIDBytes-1)))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("GenerateCaptureID() error = %v, want io.ErrUnexpectedEOF", err)
	}
	if got != "" {
		t.Errorf("GenerateCaptureID() = %q, want empty", got)
	}
}

func TestGenerateIDPropagatesRandomnessFailure(t *testing.T) {
	wantErr := errors.New("random source failed")
	got, err := GenerateID(RecordTypeFriction, iotest.ErrReader(wantErr))
	if !errors.Is(err, wantErr) {
		t.Fatalf("GenerateID() error = %v, want wrapped %v", err, wantErr)
	}
	if got != "" {
		t.Errorf("GenerateID() = %q, want empty", got)
	}
}

func TestValidateIDRejectsMalformedValues(t *testing.T) {
	tests := []struct {
		name       string
		id         ID
		recordType RecordType
	}{
		{name: "empty", id: "", recordType: RecordTypeCapture},
		{name: "wrong prefix", id: "frc_000102030405060708090a0b0c0d0e0f", recordType: RecordTypeCapture},
		{name: "short random value", id: "cap_000102030405060708090a0b0c0d0e", recordType: RecordTypeCapture},
		{name: "long random value", id: "cap_000102030405060708090a0b0c0d0e0f0", recordType: RecordTypeCapture},
		{name: "uppercase hexadecimal", id: "cap_000102030405060708090A0B0C0D0E0F", recordType: RecordTypeCapture},
		{name: "non-hexadecimal", id: "frc_000102030405060708090a0b0c0d0e0g", recordType: RecordTypeFriction},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateID(tt.id, tt.recordType)
			var invalid *InvalidValueError
			if !errors.As(err, &invalid) {
				t.Fatalf("ValidateID() error = %T %v, want *InvalidValueError", err, err)
			}
			if invalid.Field != "record ID" || invalid.Value != tt.id.String() {
				t.Errorf("error = %#v, want invalid record ID %q", invalid, tt.id)
			}
		})
	}
}

func TestValidateIDRejectsInvalidExpectedRecordType(t *testing.T) {
	err := ValidateID("cap_000102030405060708090a0b0c0d0e0f", RecordType("invalid"))
	var invalid *InvalidValueError
	if !errors.As(err, &invalid) {
		t.Fatalf("ValidateID() error = %T %v, want *InvalidValueError", err, err)
	}
	if invalid.Field != "record type" || invalid.Value != "invalid" {
		t.Errorf("error = %#v, want invalid record type", invalid)
	}
}

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("random source must not be read")
}
