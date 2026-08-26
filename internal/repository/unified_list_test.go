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

func TestListUnifiedCapturesReturnsNonNilEmptySlice(t *testing.T) {
	repository, _ := openUnifiedTestRepository(t)
	got, err := repository.ListUnifiedCaptures(context.Background(), UnifiedCaptureFilters{})
	if err != nil {
		t.Fatalf("ListUnifiedCaptures() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("ListUnifiedCaptures() = %#v, want non-nil empty slice", got)
	}
}

func TestListUnifiedCapturesOrdersNewestThenIDDescending(t *testing.T) {
	repository, _ := openUnifiedTestRepository(t)
	older := newUnifiedTestCapture(t, domain.CaptureTypeAction, 30)
	tiedLower := newUnifiedTestCapture(t, domain.CaptureTypeDecision, 31)
	tiedHigher := newUnifiedTestCapture(t, domain.CaptureTypeFriction, 32)
	olderTime := domain.NewTimestamp(time.Date(2026, time.August, 24, 9, 0, 0, 0, time.UTC))
	newerTime := domain.NewTimestamp(time.Date(2026, time.August, 25, 9, 0, 0, 0, time.UTC))
	older.CreatedAt, older.UpdatedAt = olderTime, olderTime
	tiedLower.CreatedAt, tiedLower.UpdatedAt = newerTime, newerTime
	tiedHigher.CreatedAt, tiedHigher.UpdatedAt = newerTime, newerTime
	for _, capture := range []domain.Capture{tiedLower, older, tiedHigher} {
		if err := repository.CreateUnifiedCapture(context.Background(), capture); err != nil {
			t.Fatalf("CreateUnifiedCapture() error = %v", err)
		}
	}
	got, err := repository.ListUnifiedCaptures(context.Background(), UnifiedCaptureFilters{})
	if err != nil {
		t.Fatalf("ListUnifiedCaptures() error = %v", err)
	}
	want := []domain.Capture{tiedHigher, tiedLower, older}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListUnifiedCaptures() = %#v, want %#v", got, want)
	}
}

func TestListUnifiedCapturesAppliesFiltersAndLimit(t *testing.T) {
	repository, _ := openUnifiedTestRepository(t)
	frictionForgeNew := newUnifiedTestCapture(t, domain.CaptureTypeFriction, 43)
	frictionForgeOld := newUnifiedTestCapture(t, domain.CaptureTypeFriction, 42)
	frictionOther := newUnifiedTestCapture(t, domain.CaptureTypeFriction, 41)
	otherProject := "other"
	frictionOther.Details.Friction.Project = &otherProject
	action := newUnifiedTestCapture(t, domain.CaptureTypeAction, 40)
	followUp := newUnifiedTestCapture(t, domain.CaptureTypeFollowUp, 39)
	decision := newUnifiedTestCapture(t, domain.CaptureTypeDecision, 38)
	ordered := []*domain.Capture{
		&frictionForgeNew, &frictionForgeOld, &frictionOther, &action, &followUp, &decision,
	}
	for index, capture := range ordered {
		timestamp := domain.NewTimestamp(time.Date(2026, time.August, 25, 10, len(ordered)-index, 0, 0, time.UTC))
		capture.CreatedAt, capture.UpdatedAt = timestamp, timestamp
		if err := repository.CreateUnifiedCapture(context.Background(), *capture); err != nil {
			t.Fatalf("CreateUnifiedCapture() error = %v", err)
		}
	}

	forge := "forge"
	limitOne := 1
	all := []domain.Capture{frictionForgeNew, frictionForgeOld, frictionOther, action, followUp, decision}
	tests := []struct {
		name    string
		filters UnifiedCaptureFilters
		want    []domain.Capture
	}{
		{name: "project", filters: UnifiedCaptureFilters{Project: &forge}, want: []domain.Capture{frictionForgeNew, frictionForgeOld}},
		{name: "limit", filters: UnifiedCaptureFilters{Limit: &limitOne}, want: []domain.Capture{frictionForgeNew}},
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
			filters UnifiedCaptureFilters
			want    []domain.Capture
		}{name: "type " + captureType.String(), filters: UnifiedCaptureFilters{Type: &captureType}, want: want})
	}
	frictionType := domain.CaptureTypeFriction
	tests = append(tests, struct {
		name    string
		filters UnifiedCaptureFilters
		want    []domain.Capture
	}{
		name:    "combined",
		filters: UnifiedCaptureFilters{Type: &frictionType, Project: &forge, Limit: &limitOne},
		want:    []domain.Capture{frictionForgeNew},
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repository.ListUnifiedCaptures(context.Background(), tt.filters)
			if err != nil {
				t.Fatalf("ListUnifiedCaptures() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListUnifiedCaptures() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestListUnifiedCapturesRejectsInvalidFiltersBeforeQuery(t *testing.T) {
	repository, db := openUnifiedTestRepository(t)
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
		filters UnifiedCaptureFilters
		field   string
	}{
		{name: "type", filters: UnifiedCaptureFilters{Type: &invalidType}, field: "capture type"},
		{name: "blank project", filters: UnifiedCaptureFilters{Project: &blankProject}, field: "project"},
		{name: "unnormalized project", filters: UnifiedCaptureFilters{Project: &unnormalizedProject}, field: "project"},
		{name: "zero limit", filters: UnifiedCaptureFilters{Limit: &zeroLimit}, field: "limit"},
		{name: "negative limit", filters: UnifiedCaptureFilters{Limit: &negativeLimit}, field: "limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repository.ListUnifiedCaptures(context.Background(), tt.filters)
			if got != nil {
				t.Errorf("ListUnifiedCaptures() = %#v, want nil", got)
			}
			if err == nil || !strings.Contains(err.Error(), "invalid "+tt.field) {
				t.Fatalf("ListUnifiedCaptures() error = %v, want invalid %s", err, tt.field)
			}
		})
	}
}

func TestListUnifiedCapturesMalformedIncludedAndExcludedRows(t *testing.T) {
	repository, db := openUnifiedTestRepository(t)
	valid := newUnifiedTestCapture(t, domain.CaptureTypeAction, 50)
	malformed := newUnifiedTestCapture(t, domain.CaptureTypeFriction, 51)
	for _, capture := range []domain.Capture{valid, malformed} {
		if err := repository.CreateUnifiedCapture(context.Background(), capture); err != nil {
			t.Fatalf("CreateUnifiedCapture() error = %v", err)
		}
	}
	if _, err := db.Exec(`UPDATE records SET updated_at = 'malformed' WHERE id = ?`, malformed.ID.String()); err != nil {
		t.Fatalf("corrupt stored capture error = %v", err)
	}

	got, err := repository.ListUnifiedCaptures(context.Background(), UnifiedCaptureFilters{})
	if got != nil || err == nil || !strings.Contains(err.Error(), "decode unified capture") {
		t.Fatalf("unfiltered result/error = %#v/%v, want whole-operation decode failure", got, err)
	}
	actionType := domain.CaptureTypeAction
	got, err = repository.ListUnifiedCaptures(context.Background(), UnifiedCaptureFilters{Type: &actionType})
	if err != nil {
		t.Fatalf("filtered ListUnifiedCaptures() error = %v", err)
	}
	if !reflect.DeepEqual(got, []domain.Capture{valid}) {
		t.Errorf("filtered ListUnifiedCaptures() = %#v, want valid action", got)
	}
}

func TestListUnifiedCapturesHonorsCancelledContext(t *testing.T) {
	repository, _ := openUnifiedTestRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := repository.ListUnifiedCaptures(ctx, UnifiedCaptureFilters{})
	if got != nil {
		t.Errorf("ListUnifiedCaptures() = %#v, want nil", got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListUnifiedCaptures() error = %v, want context.Canceled", err)
	}
}

func TestUnifiedCaptureListQueriesUseFilterIndexes(t *testing.T) {
	_, db := openUnifiedTestRepository(t)
	tests := []struct {
		name, predicate, argument, wantIndex string
	}{
		{name: "type", predicate: "capture_type = ?", argument: "friction", wantIndex: "idx_records_type_created"},
		{name: "project", predicate: "friction_project = ?", argument: "forge", wantIndex: "idx_records_project_created"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := db.Query(`EXPLAIN QUERY PLAN SELECT `+unifiedCaptureColumns+`
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
