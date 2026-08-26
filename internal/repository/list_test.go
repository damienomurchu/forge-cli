//go:build linux || darwin

package repository

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestListReturnsNonNilEmptySlice(t *testing.T) {
	repository, _ := openTestRepository(t)
	got, err := repository.List(context.Background(), CaptureFilters{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("List() = %#v, want non-nil empty slice", got)
	}
}

func TestListOrdersNewestThenIDDescending(t *testing.T) {
	repository, _ := openTestRepository(t)
	older := newTestCapture(t, domain.CaptureTypeAction, 30)
	tiedLower := newTestCapture(t, domain.CaptureTypeDecision, 31)
	tiedHigher := newTestCapture(t, domain.CaptureTypeFriction, 32)
	olderTime := domain.NewTimestamp(time.Date(2026, time.August, 24, 9, 0, 0, 0, time.UTC))
	newerTime := domain.NewTimestamp(time.Date(2026, time.August, 25, 9, 0, 0, 0, time.UTC))
	older.CreatedAt, older.UpdatedAt = olderTime, olderTime
	tiedLower.CreatedAt, tiedLower.UpdatedAt = newerTime, newerTime
	tiedHigher.CreatedAt, tiedHigher.UpdatedAt = newerTime, newerTime
	for _, capture := range []domain.Capture{tiedLower, older, tiedHigher} {
		if err := repository.CreateCapture(context.Background(), capture); err != nil {
			t.Fatalf("CreateCapture() error = %v", err)
		}
	}
	got, err := repository.List(context.Background(), CaptureFilters{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []domain.Capture{tiedHigher, tiedLower, older}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %#v, want %#v", got, want)
	}
}

func TestListAppliesFiltersAndLimit(t *testing.T) {
	repository, _ := openTestRepository(t)
	frictionForgeNew := newTestCapture(t, domain.CaptureTypeFriction, 43)
	frictionForgeOld := newTestCapture(t, domain.CaptureTypeFriction, 42)
	frictionOther := newTestCapture(t, domain.CaptureTypeFriction, 41)
	otherProject := "other"
	frictionOther.Details.Friction.Project = &otherProject
	action := newTestCapture(t, domain.CaptureTypeAction, 40)
	followUp := newTestCapture(t, domain.CaptureTypeFollowUp, 39)
	decision := newTestCapture(t, domain.CaptureTypeDecision, 38)
	ordered := []*domain.Capture{
		&frictionForgeNew, &frictionForgeOld, &frictionOther, &action, &followUp, &decision,
	}
	for index, capture := range ordered {
		timestamp := domain.NewTimestamp(time.Date(2026, time.August, 25, 10, len(ordered)-index, 0, 0, time.UTC))
		capture.CreatedAt, capture.UpdatedAt = timestamp, timestamp
		if err := repository.CreateCapture(context.Background(), *capture); err != nil {
			t.Fatalf("CreateCapture() error = %v", err)
		}
	}

	forge := "forge"
	limitOne := 1
	all := []domain.Capture{frictionForgeNew, frictionForgeOld, frictionOther, action, followUp, decision}
	tests := []struct {
		name    string
		filters CaptureFilters
		want    []domain.Capture
	}{
		{name: "project", filters: CaptureFilters{Project: &forge}, want: []domain.Capture{frictionForgeNew, frictionForgeOld}},
		{name: "limit", filters: CaptureFilters{Limit: &limitOne}, want: []domain.Capture{frictionForgeNew}},
	}
	for _, captureType := range domain.CaptureTypes() {
		captureType := captureType
		want := make([]domain.Capture, 0)
		for _, capture := range all {
			if capture.Type == captureType {
				want = append(want, capture)
			}
		}
		tests = append(tests, struct {
			name    string
			filters CaptureFilters
			want    []domain.Capture
		}{name: "type " + captureType.String(), filters: CaptureFilters{Type: &captureType}, want: want})
	}
	frictionType := domain.CaptureTypeFriction
	tests = append(tests, struct {
		name    string
		filters CaptureFilters
		want    []domain.Capture
	}{
		name:    "combined",
		filters: CaptureFilters{Type: &frictionType, Project: &forge, Limit: &limitOne},
		want:    []domain.Capture{frictionForgeNew},
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repository.List(context.Background(), tt.filters)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("List() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestListRejectsInvalidFiltersBeforeQuery(t *testing.T) {
	repository, db := openTestRepository(t)
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}
	invalidType := domain.CaptureType("invalid")
	blankProject := " "
	unnormalizedProject := " forge "
	zeroLimit := 0
	negativeLimit := -1
	tests := []struct {
		name    string
		filters CaptureFilters
		field   string
	}{
		{name: "type", filters: CaptureFilters{Type: &invalidType}, field: "capture type"},
		{name: "blank project", filters: CaptureFilters{Project: &blankProject}, field: "project"},
		{name: "unnormalized project", filters: CaptureFilters{Project: &unnormalizedProject}, field: "project"},
		{name: "zero limit", filters: CaptureFilters{Limit: &zeroLimit}, field: "limit"},
		{name: "negative limit", filters: CaptureFilters{Limit: &negativeLimit}, field: "limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repository.List(context.Background(), tt.filters)
			if got != nil {
				t.Errorf("List() = %#v, want nil", got)
			}
			if err == nil || !strings.Contains(err.Error(), "invalid "+tt.field) {
				t.Fatalf("List() error = %v, want invalid %s", err, tt.field)
			}
		})
	}
}

func TestListMalformedIncludedAndExcludedRows(t *testing.T) {
	repository, db := openTestRepository(t)
	valid := newTestCapture(t, domain.CaptureTypeAction, 50)
	malformed := newTestCapture(t, domain.CaptureTypeFriction, 51)
	for _, capture := range []domain.Capture{valid, malformed} {
		if err := repository.CreateCapture(context.Background(), capture); err != nil {
			t.Fatalf("CreateCapture() error = %v", err)
		}
	}
	if _, err := db.Exec(`UPDATE records SET updated_at = 'malformed' WHERE id = ?`, malformed.ID.String()); err != nil {
		t.Fatalf("corrupt stored capture error = %v", err)
	}

	got, err := repository.List(context.Background(), CaptureFilters{})
	if got != nil || err == nil || !strings.Contains(err.Error(), "decode capture") {
		t.Fatalf("unfiltered result/error = %#v/%v, want whole-operation decode failure", got, err)
	}
	actionType := domain.CaptureTypeAction
	got, err = repository.List(context.Background(), CaptureFilters{Type: &actionType})
	if err != nil {
		t.Fatalf("filtered List() error = %v", err)
	}
	if !reflect.DeepEqual(got, []domain.Capture{valid}) {
		t.Errorf("filtered List() = %#v, want valid action", got)
	}
}

func TestListHonorsCancelledContext(t *testing.T) {
	repository, _ := openTestRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := repository.List(ctx, CaptureFilters{})
	if got != nil {
		t.Errorf("List() = %#v, want nil", got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("List() error = %v, want context.Canceled", err)
	}
}

func TestCaptureListQueriesUseFilterIndexes(t *testing.T) {
	_, db := openTestRepository(t)
	tests := []struct {
		name, predicate, argument, wantIndex string
	}{
		{name: "type", predicate: "capture_type = ?", argument: "friction", wantIndex: "idx_records_type_created"},
		{name: "project", predicate: "friction_project = ?", argument: "forge", wantIndex: "idx_records_project_created"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := db.Query(`EXPLAIN QUERY PLAN SELECT `+captureColumns+`
				FROM records WHERE `+tt.predicate+` ORDER BY created_at DESC, id DESC`, tt.argument)
			if err != nil {
				t.Fatalf("explain unified list query error = %v", err)
			}
			defer rows.Close()
			var plan strings.Builder
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatalf("scan query plan error = %v", err)
				}
				plan.WriteString(detail)
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("iterate query plan error = %v", err)
			}
			if !strings.Contains(plan.String(), tt.wantIndex) {
				t.Errorf("query plan = %q, want %s", plan.String(), tt.wantIndex)
			}
		})
	}
}
