package domain

import (
	"encoding/hex"
	"fmt"
	"io"
)

const randomIDBytes = 16

const unifiedCaptureIDPrefix = "cap_"

// ID is an opaque Forge record identifier.
type ID string

func (id ID) String() string { return string(id) }

// GenerateID creates a record ID from exactly 128 random bits.
func GenerateID(recordType RecordType, random io.Reader) (ID, error) {
	prefix, err := idPrefix(recordType)
	if err != nil {
		return "", err
	}

	randomBytes := make([]byte, randomIDBytes)
	if _, err := io.ReadFull(random, randomBytes); err != nil {
		return "", fmt.Errorf("generate record ID: %w", err)
	}
	return ID(prefix + hex.EncodeToString(randomBytes)), nil
}

// GenerateCaptureID creates a unified capture ID from exactly 128 random bits.
func GenerateCaptureID(random io.Reader) (ID, error) {
	randomBytes := make([]byte, randomIDBytes)
	if _, err := io.ReadFull(random, randomBytes); err != nil {
		return "", fmt.Errorf("generate capture ID: %w", err)
	}
	return ID(unifiedCaptureIDPrefix + hex.EncodeToString(randomBytes)), nil
}

// ValidateCaptureID verifies a unified capture ID. Migration-001 friction IDs
// remain valid only for friction captures migrated into the unified model.
func ValidateCaptureID(id ID, captureType CaptureType) error {
	if !captureType.Valid() {
		return &InvalidValueError{Field: "capture type", Value: captureType.String()}
	}
	if validIDWithPrefix(id, unifiedCaptureIDPrefix) {
		return nil
	}
	if captureType == CaptureTypeFriction && validIDWithPrefix(id, "frc_") {
		return nil
	}
	return &InvalidValueError{Field: "capture ID", Value: id.String()}
}

// ValidateID verifies the generation format for the expected record type. It
// does not infer a record type from the ID.
func ValidateID(id ID, recordType RecordType) error {
	prefix, err := idPrefix(recordType)
	if err != nil {
		return err
	}

	if !validIDWithPrefix(id, prefix) {
		value := id.String()
		return &InvalidValueError{Field: "record ID", Value: value}
	}
	return nil
}

func validIDWithPrefix(id ID, prefix string) bool {
	value := id.String()
	if len(value) != len(prefix)+(randomIDBytes*2) || value[:len(prefix)] != prefix {
		return false
	}
	for _, char := range value[len(prefix):] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func idPrefix(recordType RecordType) (string, error) {
	switch recordType {
	case RecordTypeCapture:
		return "cap_", nil
	case RecordTypeFriction:
		return "frc_", nil
	default:
		return "", &InvalidValueError{Field: "record type", Value: recordType.String()}
	}
}
