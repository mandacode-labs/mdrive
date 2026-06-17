package vfs

import (
	"context"
	"testing"
	"time"

	"github.com/mandacode-labs/mdrive/internal/upload"
)

func TestInitiateAndCompleteUpload(t *testing.T) {
	svc := testService()
	ctx := context.Background()

	info, err := svc.InitiateUpload(ctx, "user1", "d1", "/big.bin", strPtr("application/octet-stream"), int64Ptr(42), time.Hour)
	if err != nil {
		t.Fatalf("initiate upload: %v", err)
	}
	if info.Method != "PUT" {
		t.Errorf("expected PUT, got %s", info.Method)
	}
	if info.UploadID == "" {
		t.Error("expected uploadID")
	}

	// FakeStore returns empty URLs; just verify the registry round-trip.
	meta, err := svc.Reg.Get(ctx, info.UploadID)
	if err != nil {
		t.Fatalf("registry get: %v", err)
	}
	if meta.Path != "/big.bin" {
		t.Errorf("expected path /big.bin, got %s", meta.Path)
	}

	n, err := svc.CompleteUpload(ctx, "user1", "d1", info.UploadID, 42, nil)
	if err != nil {
		t.Fatalf("complete upload: %v", err)
	}
	if !n.IsObject() {
		t.Error("expected object node")
	}

	// Token should be deleted after completion.
	if _, err := svc.Reg.Get(ctx, info.UploadID); err != upload.ErrNotFound {
		t.Fatalf("expected token deleted, got %v", err)
	}
}

func TestCompleteUploadSizeMismatch(t *testing.T) {
	svc := testService()
	ctx := context.Background()

	info, _ := svc.InitiateUpload(ctx, "user1", "d1", "/big.bin", nil, int64Ptr(42), time.Hour)
	if _, err := svc.CompleteUpload(ctx, "user1", "d1", info.UploadID, 43, nil); err == nil {
		t.Fatal("expected size mismatch error")
	}
}

func TestPresignDownloadNotObject(t *testing.T) {
	svc := testService()
	ctx := context.Background()
	_, _ = svc.Touch(ctx, "user1", "d1", "/plain.txt")
	if _, err := svc.PresignDownload(ctx, "user1", "d1", "/plain.txt", time.Hour); err == nil {
		t.Fatal("expected error for non-object node")
	}
}

func strPtr(s string) *string  { return &s }
func int64Ptr(i int64) *int64 { return &i }
