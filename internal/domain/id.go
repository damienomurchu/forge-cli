package domain

import (
	"encoding/hex"
	"fmt"
	"io"
)

const randomIDBytes = 16
const captureIDPrefix = "cap_"

// ID is an opaque Forge capture identifier.
type ID string

func (id ID) String() string { return string(id) }

// GenerateCaptureID creates a capture ID from exactly 128 random bits.
func GenerateCaptureID(random io.Reader) (ID, error) {
	randomBytes := make([]byte, randomIDBytes)
	if _, err := io.ReadFull(random, randomBytes); err != nil {
		return "", fmt.Errorf("generate capture ID: %w", err)
	}
	return ID(captureIDPrefix + hex.EncodeToString(randomBytes)), nil
}

// ValidateCaptureID verifies a capture ID. Migration-001 friction IDs remain
// valid only for friction captures migrated into the unified model.
func ValidateCaptureID(id ID, captureType CaptureType) error {
	if !captureType.Valid() {
		return &InvalidValueError{Field: "capture type", Value: captureType.String()}
	}
	if validIDWithPrefix(id, captureIDPrefix) {
		return nil
	}
	if captureType == CaptureTypeFriction && validIDWithPrefix(id, "frc_") {
		return nil
	}
	return &InvalidValueError{Field: "capture ID", Value: id.String()}
}

func validIDWithPrefix(id ID, prefix string) bool {
	value := id.String()
	if len(value) != len(prefix)+(randomIDBytes*2) || value[:len(prefix)] != prefix {
		return false
	}
	for _, character := range value[len(prefix):] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
