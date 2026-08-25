package domain

import (
	"encoding/hex"
	"fmt"
	"io"
)

const randomIDBytes = 16

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

// ValidateID verifies the generation format for the expected record type. It
// does not infer a record type from the ID.
func ValidateID(id ID, recordType RecordType) error {
	prefix, err := idPrefix(recordType)
	if err != nil {
		return err
	}

	value := id.String()
	if len(value) != len(prefix)+(randomIDBytes*2) || value[:len(prefix)] != prefix {
		return &InvalidValueError{Field: "record ID", Value: value}
	}
	for _, char := range value[len(prefix):] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return &InvalidValueError{Field: "record ID", Value: value}
		}
	}
	return nil
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
