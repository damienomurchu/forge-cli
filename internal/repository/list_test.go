//go:build linux || darwin

package repository

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestListReturnsNonNilEmptySlice(t *testing.T) {
	repository, _ := openTestRepository(t)
	records, err := repository.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if records == nil || len(records) != 0 {
		t.Errorf("List() = %#v, want non-nil empty slice", records)
	}
}

func TestListReturnsMixedRecordsNewestFirst(t *testing.T) {
	repository, _ := openTestRepository(t)
	older := newTestCapture(t, domain.CaptureInput{
		Description: "Older capture",
		Kind:        domain.CaptureKindThought,
		Tags:        "older,ordered",
	}, 30)
	newerCapture := newTestCapture(t, domain.CaptureInput{
		Description: "Newer capture",
		Kind:        domain.CaptureKindObservation,
		Tags:        "first,second",
	}, 31)
	newerFriction := newTestFriction(t, domain.FrictionInput{
		Description: "Newer friction",
		Frequency:   domain.FrequencyDaily,
		Impact:      domain.ImpactHigh,
		Category:    domain.CategoryContextSwitching,
	}, 32)

	olderTime := domain.NewTimestamp(time.Date(2026, time.August, 24, 9, 0, 0, 0, time.UTC))
	newerTime := domain.NewTimestamp(time.Date(2026, time.August, 25, 9, 0, 0, 0, time.UTC))
	older.CreatedAt, older.UpdatedAt = olderTime, olderTime
	newerCapture.CreatedAt, newerCapture.UpdatedAt = newerTime, newerTime
	newerFriction.CreatedAt, newerFriction.UpdatedAt = newerTime, newerTime

	if err := repository.CreateCapture(context.Background(), newerCapture); err != nil {
		t.Fatalf("CreateCapture(newer) error = %v", err)
	}
	if err := repository.CreateCapture(context.Background(), older); err != nil {
		t.Fatalf("CreateCapture(older) error = %v", err)
	}
	if err := repository.CreateFriction(context.Background(), newerFriction); err != nil {
		t.Fatalf("CreateFriction() error = %v", err)
	}

	got, err := repository.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []domain.Record{newerFriction, newerCapture, older}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %#v, want %#v", got, want)
	}
}

func TestListFailsWholeOperationForMalformedRecord(t *testing.T) {
	repository, db := openTestRepository(t)
	valid := newTestCapture(t, domain.CaptureInput{
		Description: "Valid capture",
		Kind:        domain.CaptureKindIdea,
	}, 33)
	malformed := newTestFriction(t, domain.FrictionInput{
		Description: "Malformed friction",
		Frequency:   domain.FrequencyMonthly,
		Impact:      domain.ImpactMedium,
		Category:    domain.CategoryVerification,
	}, 34)
	if err := repository.CreateCapture(context.Background(), valid); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}
	if err := repository.CreateFriction(context.Background(), malformed); err != nil {
		t.Fatalf("CreateFriction() error = %v", err)
	}
	if _, err := db.Exec(
		`UPDATE records SET updated_at = 'malformed' WHERE id = ?`,
		malformed.ID.String(),
	); err != nil {
		t.Fatalf("corrupt timestamp error = %v", err)
	}

	got, err := repository.List(context.Background(), ListOptions{})
	if got != nil {
		t.Errorf("List() = %#v, want nil after failure", got)
	}
	if err == nil || !strings.Contains(err.Error(), "decode listed record") {
		t.Fatalf("List() error = %v, want decode error", err)
	}
}

func TestListAppliesFiltersAndLimit(t *testing.T) {
	repository, _ := openTestRepository(t)
	newestCapture := newTestCapture(t, domain.CaptureInput{
		Description: "Newest Forge capture",
		Project:     "forge",
		Kind:        domain.CaptureKindIdea,
		Tags:        "filtered,ordered",
	}, 43)
	forgeFriction := newTestFriction(t, domain.FrictionInput{
		Description: "Forge friction under review",
		Project:     "forge",
		Frequency:   domain.FrequencyWeekly,
		Impact:      domain.ImpactHigh,
		Category:    domain.CategoryVerification,
	}, 42)
	forgeFriction.Status = domain.StatusReviewing
	otherCapture := newTestCapture(t, domain.CaptureInput{
		Description: "Other project capture",
		Project:     "other",
		Kind:        domain.CaptureKindQuestion,
	}, 41)
	oldestFriction := newTestFriction(t, domain.FrictionInput{
		Description: "Unassigned friction",
		Frequency:   domain.FrequencyUnknown,
		Impact:      domain.ImpactUnknown,
		Category:    domain.CategoryOther,
	}, 40)
	ordered := []struct {
		record *domain.Record
		minute int
	}{
		{record: &newestCapture, minute: 4},
		{record: &forgeFriction, minute: 3},
		{record: &otherCapture, minute: 2},
		{record: &oldestFriction, minute: 1},
	}
	for _, item := range ordered {
		timestamp := domain.NewTimestamp(time.Date(2026, time.August, 25, 10, item.minute, 0, 0, time.UTC))
		item.record.CreatedAt, item.record.UpdatedAt = timestamp, timestamp
	}

	for _, record := range []domain.Record{otherCapture, forgeFriction, oldestFriction, newestCapture} {
		var err error
		if record.Type == domain.RecordTypeCapture {
			err = repository.CreateCapture(context.Background(), record)
		} else {
			err = repository.CreateFriction(context.Background(), record)
		}
		if err != nil {
			t.Fatalf("store %s error = %v", record.ID, err)
		}
	}

	captureType := domain.RecordTypeCapture
	frictionType := domain.RecordTypeFriction
	forgeProject := "forge"
	reviewingStatus := domain.StatusReviewing
	limitTwo := 2
	tests := []struct {
		name    string
		options ListOptions
		want    []domain.Record
	}{
		{name: "type", options: ListOptions{Type: &captureType}, want: []domain.Record{newestCapture, otherCapture}},
		{name: "project", options: ListOptions{Project: &forgeProject}, want: []domain.Record{newestCapture, forgeFriction}},
		{name: "status", options: ListOptions{Status: &reviewingStatus}, want: []domain.Record{forgeFriction}},
		{
			name: "combined",
			options: ListOptions{
				Type:    &frictionType,
				Project: &forgeProject,
				Status:  &reviewingStatus,
			},
			want: []domain.Record{forgeFriction},
		},
		{name: "limit after ordering", options: ListOptions{Limit: &limitTwo}, want: []domain.Record{newestCapture, forgeFriction}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repository.List(context.Background(), tt.options)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("List() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestListRejectsInvalidOptionsBeforeQuerying(t *testing.T) {
	repository, db := openTestRepository(t)
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}
	invalidType := domain.RecordType("invalid")
	blankProject := " "
	unnormalizedProject := " forge "
	invalidStatus := domain.Status("invalid")
	zeroLimit := 0
	negativeLimit := -1
	tests := []struct {
		name    string
		options ListOptions
		field   string
	}{
		{name: "type", options: ListOptions{Type: &invalidType}, field: "record type"},
		{name: "blank project", options: ListOptions{Project: &blankProject}, field: "project"},
		{name: "unnormalized project", options: ListOptions{Project: &unnormalizedProject}, field: "project"},
		{name: "status", options: ListOptions{Status: &invalidStatus}, field: "status"},
		{name: "zero limit", options: ListOptions{Limit: &zeroLimit}, field: "limit"},
		{name: "negative limit", options: ListOptions{Limit: &negativeLimit}, field: "limit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repository.List(context.Background(), tt.options)
			if got != nil {
				t.Errorf("List() = %#v, want nil", got)
			}
			if err == nil || !strings.Contains(err.Error(), "invalid "+tt.field) {
				t.Fatalf("List() error = %v, want invalid %s", err, tt.field)
			}
		})
	}
}
