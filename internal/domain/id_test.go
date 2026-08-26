package domain

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"
)

func TestGenerateCaptureID(t *testing.T) {
	got, err := GenerateCaptureID(bytes.NewReader([]byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	}))
	if err != nil {
		t.Fatalf("GenerateCaptureID() error = %v", err)
	}
	if want := ID("cap_000102030405060708090a0b0c0d0e0f"); got != want || got.String() != string(want) {
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
			t.Errorf("ValidateCaptureID(%q, %q) error = %T %v, want invalid capture ID", legacy, captureType, err, err)
		}
	}
}

func TestValidateCaptureIDRejectsMalformedValues(t *testing.T) {
	values := []ID{
		"", "cap_000102030405060708090a0b0c0d0e",
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

func TestValidateCaptureIDRejectsInvalidCaptureType(t *testing.T) {
	err := ValidateCaptureID("cap_000102030405060708090a0b0c0d0e0f", "invalid")
	var invalid *InvalidValueError
	if !errors.As(err, &invalid) || invalid.Field != "capture type" {
		t.Fatalf("error = %T %v, want invalid capture type", err, err)
	}
}

func TestGenerateCaptureIDRequiresExactly128Bits(t *testing.T) {
	got, err := GenerateCaptureID(bytes.NewReader(make([]byte, randomIDBytes-1)))
	if !errors.Is(err, io.ErrUnexpectedEOF) || got != "" {
		t.Fatalf("ID/error = %q/%v, want empty/io.ErrUnexpectedEOF", got, err)
	}
}

func TestGenerateCaptureIDPropagatesRandomnessFailure(t *testing.T) {
	wantErr := errors.New("random source failed")
	got, err := GenerateCaptureID(iotest.ErrReader(wantErr))
	if !errors.Is(err, wantErr) || got != "" {
		t.Fatalf("ID/error = %q/%v, want empty/%v", got, err, wantErr)
	}
}
