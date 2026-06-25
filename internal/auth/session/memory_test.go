package session

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStoreCreateGetDelete(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	sess := New(1 * time.Hour)

	err := s.Create(ctx, sess)
	require.NoError(t, err)

	got, err := s.Get(ctx, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, sess.ID, got.ID)
	assert.Equal(t, sess.ExpiresAt.Unix(), got.ExpiresAt.Unix())

	err = s.Delete(ctx, sess.ID)
	require.NoError(t, err)

	_, err = s.Get(ctx, sess.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestMemoryStoreGetNotFound(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.Get(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestMemoryStoreGetExpired(t *testing.T) {
	s := NewMemoryStore()
	sess := New(-1 * time.Hour) // already expired
	require.NoError(t, s.Create(context.Background(), sess))

	_, err := s.Get(context.Background(), sess.ID)
	assert.ErrorIs(t, err, ErrExpired)

	// Expired session should be cleaned up.
	_, err = s.Get(context.Background(), sess.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestMemoryStoreDeleteNotFound(t *testing.T) {
	s := NewMemoryStore()
	err := s.Delete(context.Background(), "nonexistent")
	assert.NoError(t, err)
}

func TestSessionEncodeDecode(t *testing.T) {
	sess := New(2 * time.Hour)
	sess.UserID = "user123"
	sess.Provider = "google"

	data, err := sess.Encode()
	require.NoError(t, err)

	got, err := Decode(data)
	require.NoError(t, err)
	assert.Equal(t, sess.ID, got.ID)
	assert.Equal(t, "user123", got.UserID)
	assert.Equal(t, "google", got.Provider)
}

func TestSessionIsExpired(t *testing.T) {
	assert.True(t, New(-1*time.Second).IsExpired())
	assert.False(t, New(1*time.Hour).IsExpired())
}

func TestSessionTTL(t *testing.T) {
	sess := New(1 * time.Hour)
	assert.InDelta(t, 1*time.Hour, sess.TTL(), float64(2*time.Second))
}
