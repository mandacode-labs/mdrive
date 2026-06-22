package cryptostore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mandacode-labs/mdrive/internal/crypto"
)

// CipherLookup returns the NodeCipher for the given driveID, or
// (nil, nil) for drives without a DEK (predates Phase 3b) or when
// envelope encryption is disabled. The cryptostore treats the two
// no-cipher cases the same: it falls back to a pass-through write
// to the inner store.
type CipherLookup func(ctx context.Context, driveID string) (*crypto.NodeCipher, error)

// Store wraps a vfs.Store with per-drive envelope encryption for
// object bodies. The implementation satisfies vfs.Store, so callers
// can swap it in transparently for the raw S3 client.
//
// DriveID is parsed from the object key prefix ("drives/{driveID}/...")
// — the vfs upload flow already uses this convention, so no interface
// change is required. Keys that do not match the prefix are stored
// as plaintext, which preserves backwards compatibility for any
// out-of-band objects.
type Store struct {
	inner        Inner
	cipherLookup CipherLookup
}

// Inner is the subset of vfs.Store the cryptostore actually depends
// on. Defining it here (rather than importing vfs.Store) keeps the
// dependency graph small and makes the cryptostore unit-testable
// against an in-memory fake.
type Inner interface {
	PutObject(ctx context.Context, bucket, key string, reader io.Reader, size int64) error
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	DeleteObjects(ctx context.Context, bucket string, keys []string) error
	ObjectExists(ctx context.Context, bucket, key string) (bool, error)
	GetObjectSize(ctx context.Context, bucket, key string) (int64, error)
	GetObjectChecksum(ctx context.Context, bucket, key string) (string, error)
	GetPresignedUploadURL(ctx context.Context, bucket, key, contentType string, size int64, checksum string, expiry time.Duration) (string, error)
	GetPresignedDownloadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
}

// New wraps inner with envelope encryption. cipherLookup may be nil
// (no encryption; equivalent to the inner store).
func New(inner Inner, cipherLookup CipherLookup) *Store {
	return &Store{inner: inner, cipherLookup: cipherLookup}
}

// PutObject encrypts the body and writes the ciphertext to inner.
// The plaintext is never persisted. If cipherLookup is nil or the
// key does not match the driveID prefix, the body is written
// as-is (pass-through).
func (s *Store) PutObject(ctx context.Context, bucket, key string, reader io.Reader, _ int64) error {
	driveID, ok := driveIDFromKey(key)
	if !ok {
		return s.inner.PutObject(ctx, bucket, key, reader, 0)
	}
	cipher, err := s.cipherFor(ctx, driveID)
	if err != nil {
		return err
	}
	if cipher == nil {
		return s.inner.PutObject(ctx, bucket, key, reader, 0)
	}
	aad := objectAAD(driveID, bucket, key)
	var buf bytes.Buffer
	if _, err := encodeStreaming(cipher, reader, &buf, aad); err != nil {
		return err
	}
	return s.inner.PutObject(ctx, bucket, key, bytes.NewReader(buf.Bytes()), int64(buf.Len()))
}

// GetObject reads the ciphertext from inner and decrypts it. If the
// key has no driveID prefix or no cipher is available, the bytes
// are returned as-is.
func (s *Store) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	ct, err := s.inner.GetObject(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	driveID, ok := driveIDFromKey(key)
	if !ok {
		return ct, nil
	}
	cipher, err := s.cipherFor(ctx, driveID)
	if err != nil {
		return nil, err
	}
	if cipher == nil {
		return ct, nil
	}
	aad := objectAAD(driveID, bucket, key)
	var out bytes.Buffer
	if err := decodeStreaming(cipher, bytes.NewReader(ct), &out, aad); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// MigratePlaintext reads a plaintext object from srcKey, encrypts it,
// writes the ciphertext to dstKey, and deletes the plaintext. Used
// by vfs.CompleteUpload to re-encrypt objects that clients uploaded
// directly to S3 via a presigned URL.
func (s *Store) MigratePlaintext(ctx context.Context, driveID, bucket, srcKey, dstKey string) error {
	cipher, err := s.cipherFor(ctx, driveID)
	if err != nil {
		return err
	}
	if cipher == nil {
		return nil
	}
	plaintext, err := s.inner.GetObject(ctx, bucket, srcKey)
	if err != nil {
		return fmt.Errorf("cryptostore: migrate: read plaintext: %w", err)
	}
	aad := objectAAD(driveID, bucket, dstKey)
	var buf bytes.Buffer
	if _, err := encodeStreaming(cipher, bytes.NewReader(plaintext), &buf, aad); err != nil {
		return fmt.Errorf("cryptostore: migrate: encrypt: %w", err)
	}
	if err := s.inner.PutObject(ctx, bucket, dstKey, bytes.NewReader(buf.Bytes()), int64(buf.Len())); err != nil {
		return fmt.Errorf("cryptostore: migrate: write ciphertext: %w", err)
	}
	if err := s.inner.DeleteObject(ctx, bucket, srcKey); err != nil {
		return fmt.Errorf("cryptostore: migrate: delete plaintext: %w", err)
	}
	return nil
}

// Pass-throughs (no encryption needed).

func (s *Store) DeleteObject(ctx context.Context, bucket, key string) error {
	return s.inner.DeleteObject(ctx, bucket, key)
}

func (s *Store) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	return s.inner.DeleteObjects(ctx, bucket, keys)
}

func (s *Store) ObjectExists(ctx context.Context, bucket, key string) (bool, error) {
	return s.inner.ObjectExists(ctx, bucket, key)
}

func (s *Store) GetObjectSize(ctx context.Context, bucket, key string) (int64, error) {
	return s.inner.GetObjectSize(ctx, bucket, key)
}

func (s *Store) GetObjectChecksum(ctx context.Context, bucket, key string) (string, error) {
	return s.inner.GetObjectChecksum(ctx, bucket, key)
}

func (s *Store) GetPresignedUploadURL(ctx context.Context, bucket, key, contentType string, size int64, checksum string, expiry time.Duration) (string, error) {
	return s.inner.GetPresignedUploadURL(ctx, bucket, key, contentType, size, checksum, expiry)
}

func (s *Store) GetPresignedDownloadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	return s.inner.GetPresignedDownloadURL(ctx, bucket, key, expiry)
}

// cipherFor resolves the per-drive cipher. Returns (nil, nil) when
// cipherLookup is nil or the drive has no DEK.
func (s *Store) cipherFor(ctx context.Context, driveID string) (*crypto.NodeCipher, error) {
	if s.cipherLookup == nil {
		return nil, nil
	}
	return s.cipherLookup(ctx, driveID)
}

// driveIDFromKey parses the vfs upload key prefix
// ("drives/{driveID}/...") and returns the driveID. Returns
// ("", false) when the prefix is absent, signalling to callers that
// the object should be treated as plaintext.
func driveIDFromKey(key string) (string, bool) {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) < 3 || parts[0] != "drives" || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

// objectAAD is the additional data bound to every object-body
// ciphertext. Encoding: driveID || ":" || bucket || ":" || key.
// Including bucket+key in the AAD prevents a ciphertext written
// against one object from being replayed against another object
// in the same drive (or another drive that happens to share the
// driveID in the key).
func objectAAD(driveID, bucket, key string) []byte {
	out := make([]byte, 0, len(driveID)+1+len(bucket)+1+len(key))
	out = append(out, driveID...)
	out = append(out, ':')
	out = append(out, bucket...)
	out = append(out, ':')
	out = append(out, key...)
	return out
}
