//go:build linux || darwin

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestDeleteByIDDeletesExactlyOneCapture(t *testing.T) {
	repository, _ := openTestRepository(t)
	capture := newTestCapture(t, domain.CaptureTypeAction, 31)
	if err := repository.CreateCapture(context.Background(), capture); err != nil {
		t.Fatal(err)
	}
	if err := repository.DeleteByID(context.Background(), capture.ID); err != nil {
		t.Fatalf("DeleteByID() error = %v", err)
	}
	if _, err := repository.FindByID(context.Background(), capture.ID); !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("FindByID() after delete error = %v, want ErrRecordNotFound", err)
	}
}

func TestDeleteByIDReturnsStableNotFoundError(t *testing.T) {
	repository, _ := openTestRepository(t)
	if err := repository.DeleteByID(context.Background(), "missing"); !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("DeleteByID() error = %v, want ErrRecordNotFound", err)
	}
}

func TestDeleteByIDHonorsCancelledContext(t *testing.T) {
	repository, _ := openTestRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := repository.DeleteByID(ctx, "missing"); !errors.Is(err, context.Canceled) {
		t.Fatalf("DeleteByID() error = %v, want context.Canceled", err)
	}
}
