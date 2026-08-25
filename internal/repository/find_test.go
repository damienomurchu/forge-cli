//go:build linux || darwin

package repository

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestFindByIDHydratesCapture(t *testing.T) {
	repository, _ := openTestRepository(t)
	want := newTestCapture(t, domain.CaptureInput{
		Description: "Hydrate this capture",
		Project:     "forge",
		Kind:        domain.CaptureKindDecision,
		Tags:        "third,first,second",
	}, 20)
	if err := repository.CreateCapture(context.Background(), want); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}

	got, err := repository.FindByID(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FindByID() = %#v, want %#v", got, want)
	}
}

func TestFindByIDHydratesFriction(t *testing.T) {
	repository, _ := openTestRepository(t)
	want := newTestFriction(t, domain.FrictionInput{
		Description:       "Hydrate this friction",
		Project:           "forge",
		Frequency:         domain.FrequencyWeekly,
		Impact:            domain.ImpactHigh,
		Category:          domain.CategoryContextSwitching,
		CurrentWorkaround: "Keep two windows open",
	}, 21)
	if err := repository.CreateFriction(context.Background(), want); err != nil {
		t.Fatalf("CreateFriction() error = %v", err)
	}

	got, err := repository.FindByID(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FindByID() = %#v, want %#v", got, want)
	}
}

func TestFindByIDPreservesAbsentOptionalValues(t *testing.T) {
	repository, _ := openTestRepository(t)
	capture := newTestCapture(t, domain.CaptureInput{
		Description: "No capture optionals",
		Kind:        domain.CaptureKindThought,
	}, 22)
	friction := newTestFriction(t, domain.FrictionInput{
		Description: "No friction optionals",
		Frequency:   domain.FrequencyUnknown,
		Impact:      domain.ImpactUnknown,
		Category:    domain.CategoryOther,
	}, 23)
	if err := repository.CreateCapture(context.Background(), capture); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}
	if err := repository.CreateFriction(context.Background(), friction); err != nil {
		t.Fatalf("CreateFriction() error = %v", err)
	}

	gotCapture, err := repository.FindByID(context.Background(), capture.ID)
	if err != nil {
		t.Fatalf("FindByID(capture) error = %v", err)
	}
	gotFriction, err := repository.FindByID(context.Background(), friction.ID)
	if err != nil {
		t.Fatalf("FindByID(friction) error = %v", err)
	}
	if !reflect.DeepEqual(gotCapture, capture) || !reflect.DeepEqual(gotFriction, friction) {
		t.Errorf("absent optional values did not round trip")
	}
}

func TestFindByIDReturnsStableNotFoundError(t *testing.T) {
	repository, _ := openTestRepository(t)
	got, err := repository.FindByID(context.Background(), domain.ID("missing"))
	if got != (domain.Record{}) {
		t.Errorf("FindByID() = %#v, want zero record", got)
	}
	if !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("FindByID() error = %v, want ErrRecordNotFound", err)
	}
}

func TestFindByIDRejectsMalformedStoredTimestamp(t *testing.T) {
	repository, db := openTestRepository(t)
	record := newTestFriction(t, domain.FrictionInput{
		Description: "Corrupt timestamp",
		Frequency:   domain.FrequencyDaily,
		Impact:      domain.ImpactLow,
		Category:    domain.CategoryWaiting,
	}, 24)
	if err := repository.CreateFriction(context.Background(), record); err != nil {
		t.Fatalf("CreateFriction() error = %v", err)
	}
	if _, err := db.Exec(`UPDATE records SET created_at = 'not-a-timestamp' WHERE id = ?`, record.ID.String()); err != nil {
		t.Fatalf("corrupt timestamp error = %v", err)
	}

	got, err := repository.FindByID(context.Background(), record.ID)
	if got != (domain.Record{}) {
		t.Errorf("FindByID() = %#v, want zero record", got)
	}
	if err == nil || !strings.Contains(err.Error(), "decode stored record") || errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("FindByID() error = %v, want stored-data error", err)
	}
}

func TestFindByIDRejectsNoncontiguousCaptureTags(t *testing.T) {
	repository, db := openTestRepository(t)
	record := newTestCapture(t, domain.CaptureInput{
		Description: "Corrupt tag positions",
		Kind:        domain.CaptureKindObservation,
		Tags:        "first,second",
	}, 25)
	if err := repository.CreateCapture(context.Background(), record); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}
	if _, err := db.Exec(
		`UPDATE record_tags SET position = position + 2 WHERE record_id = ?`,
		record.ID.String(),
	); err != nil {
		t.Fatalf("corrupt tag positions error = %v", err)
	}

	got, err := repository.FindByID(context.Background(), record.ID)
	if got != (domain.Record{}) {
		t.Errorf("FindByID() = %#v, want zero record", got)
	}
	if err == nil || !strings.Contains(err.Error(), "tag position 2 is not contiguous") {
		t.Fatalf("FindByID() error = %v, want tag-position error", err)
	}
}
