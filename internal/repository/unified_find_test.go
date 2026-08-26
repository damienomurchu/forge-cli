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

func TestFindUnifiedCaptureByIDRoundTripsEveryType(t *testing.T) {
	for index, captureType := range domain.CaptureTypes() {
		t.Run(captureType.String(), func(t *testing.T) {
			repository, _ := openUnifiedTestRepository(t)
			want := newUnifiedTestCapture(t, captureType, byte(index+20))
			if err := repository.CreateUnifiedCapture(context.Background(), want); err != nil {
				t.Fatalf("CreateUnifiedCapture() error = %v", err)
			}
			got, err := repository.FindUnifiedCaptureByID(context.Background(), want.ID)
			if err != nil {
				t.Fatalf("FindUnifiedCaptureByID() error = %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("FindUnifiedCaptureByID() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestFindUnifiedCaptureByIDPreservesAbsentFrictionText(t *testing.T) {
	repository, _ := openUnifiedTestRepository(t)
	want := newUnifiedTestCapture(t, domain.CaptureTypeFriction, 24)
	want.Details.Friction.Project = nil
	want.Details.Friction.CurrentWorkaround = nil
	if err := repository.CreateUnifiedCapture(context.Background(), want); err != nil {
		t.Fatalf("CreateUnifiedCapture() error = %v", err)
	}
	got, err := repository.FindUnifiedCaptureByID(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("FindUnifiedCaptureByID() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FindUnifiedCaptureByID() = %#v, want %#v", got, want)
	}
}

func TestFindUnifiedCaptureByIDSupportsMigratedFrictionID(t *testing.T) {
	repository, _ := openUnifiedTestRepository(t)
	want := newUnifiedTestCapture(t, domain.CaptureTypeFriction, 25)
	want.ID = "frc_19191919191919191919191919191919"
	if err := repository.CreateUnifiedCapture(context.Background(), want); err != nil {
		t.Fatalf("CreateUnifiedCapture() error = %v", err)
	}
	got, err := repository.FindUnifiedCaptureByID(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("FindUnifiedCaptureByID() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FindUnifiedCaptureByID() = %#v, want %#v", got, want)
	}
}

func TestFindUnifiedCaptureByIDReturnsStableNotFoundError(t *testing.T) {
	repository, _ := openUnifiedTestRepository(t)
	got, err := repository.FindUnifiedCaptureByID(context.Background(), "missing")
	if got != (domain.Capture{}) {
		t.Errorf("FindUnifiedCaptureByID() = %#v, want zero capture", got)
	}
	if !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("FindUnifiedCaptureByID() error = %v, want ErrRecordNotFound", err)
	}
}

func TestFindUnifiedCaptureByIDRejectsMalformedStoredData(t *testing.T) {
	tests := []struct {
		name   string
		update string
	}{
		{name: "capture type", update: `UPDATE records SET capture_type = 'thought'`},
		{name: "frequency", update: `UPDATE records SET friction_frequency = 'often'`},
		{name: "missing friction column", update: `UPDATE records SET friction_impact = NULL`},
		{name: "mismatched columns", update: `UPDATE records SET capture_type = 'action'`},
		{name: "created timestamp", update: `UPDATE records SET created_at = 'invalid'`},
		{name: "updated timestamp", update: `UPDATE records SET updated_at = 'invalid'`},
		{name: "unnormalized project", update: `UPDATE records SET friction_project = ' forge '`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository, db := openUnifiedTestRepository(t)
			capture := newUnifiedTestCapture(t, domain.CaptureTypeFriction, 26)
			if err := repository.CreateUnifiedCapture(context.Background(), capture); err != nil {
				t.Fatalf("CreateUnifiedCapture() error = %v", err)
			}
			if _, err := db.Exec(`PRAGMA ignore_check_constraints = ON`); err != nil {
				t.Fatalf("disable check constraints error = %v", err)
			}
			if _, err := db.Exec(tt.update+` WHERE id = ?`, capture.ID.String()); err != nil {
				t.Fatalf("corrupt stored capture error = %v", err)
			}
			if _, err := db.Exec(`PRAGMA ignore_check_constraints = OFF`); err != nil {
				t.Fatalf("enable check constraints error = %v", err)
			}

			got, err := repository.FindUnifiedCaptureByID(context.Background(), capture.ID)
			if got != (domain.Capture{}) {
				t.Errorf("FindUnifiedCaptureByID() = %#v, want zero capture", got)
			}
			if err == nil || !strings.Contains(err.Error(), "decode stored unified capture") || errors.Is(err, ErrRecordNotFound) {
				t.Fatalf("FindUnifiedCaptureByID() error = %v, want stored-data error", err)
			}
		})
	}
}

func TestFindUnifiedCaptureByIDHonorsCancelledContext(t *testing.T) {
	repository, _ := openUnifiedTestRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := repository.FindUnifiedCaptureByID(ctx, "missing")
	if got != (domain.Capture{}) {
		t.Errorf("FindUnifiedCaptureByID() = %#v, want zero capture", got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FindUnifiedCaptureByID() error = %v, want context.Canceled", err)
	}
}
