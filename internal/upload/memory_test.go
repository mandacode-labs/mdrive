package upload

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, r.Put(ctx, meta, time.Hour))

	got, err := r.Get(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, meta.UploadID, got.UploadID)

	require.NoError(t, r.Delete(ctx, "u1"))
	_, err = r.Get(ctx, "u1")
	assert.ErrorIs(t, err, ErrNotFound)
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
	require.NoError(t, r.Put(ctx, meta, time.Millisecond))
	time.Sleep(5 * time.Millisecond)
	_, err := r.Get(ctx, "u1")
	assert.ErrorIs(t, err, ErrNotFound)
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
	require.NoError(t, err)
	got, err := DecodePresignMeta(data)
	require.NoError(t, err)
	assert.Equal(t, ct, *got.ContentType)
	assert.Equal(t, size, *got.Size)
}