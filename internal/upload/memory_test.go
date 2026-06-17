package upload

import (
	"context"
	"testing"
	"time"
)

func TestMemoryRegistryRoundTrip(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRegistry()
	meta := PresignMeta{
		UploadID:  "u1",
		DriveID:   "d1",
		UserID:    "user1",
		Path:      "/file.txt",
		Bucket:    "b",
		Key:       "k",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := r.Put(ctx, meta, time.Hour); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := r.Get(ctx, "u1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.UploadID != meta.UploadID {
		t.Errorf("expected uploadID %q, got %q", meta.UploadID, got.UploadID)
	}
	if err := r.Delete(ctx, "u1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.Get(ctx, "u1"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryRegistryExpiry(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRegistry()
	meta := PresignMeta{
		UploadID:  "u1",
		DriveID:   "d1",
		Bucket:    "b",
		Key:       "k",
		ExpiresAt: time.Now().Add(-time.Second),
	}
	if err := r.Put(ctx, meta, time.Millisecond); err != nil {
		t.Fatalf("put: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := r.Get(ctx, "u1"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after expiry, got %v", err)
	}
}

func TestPresignMetaEncodeDecode(t *testing.T) {
	ct := "text/plain"
	size := int64(42)
	meta := PresignMeta{
		UploadID:    "u1",
		DriveID:     "d1",
		UserID:      "user1",
		Path:        "/file.txt",
		Bucket:      "b",
		Key:         "k",
		ContentType: &ct,
		Size:        &size,
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	data, err := meta.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodePresignMeta(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if *got.ContentType != ct {
		t.Errorf("content type mismatch")
	}
	if *got.Size != size {
		t.Errorf("size mismatch")
	}
}
