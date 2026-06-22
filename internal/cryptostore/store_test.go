package cryptostore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mandacode-labs/mdrive/internal/crypto"
)

// memInner is an in-memory Inner used by tests.
type memInner struct {
	objects map[string][]byte
}

func newMemInner() *memInner { return &memInner{objects: map[string][]byte{}} }

func (m *memInner) PutObject(_ context.Context, _, key string, r io.Reader, _ int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.objects[key] = data
	return nil
}
func (m *memInner) GetObject(_ context.Context, _, key string) ([]byte, error) {
	v, ok := m.objects[key]
	if !ok {
		return nil, errors.New("memInner: not found")
	}
	return v, nil
}
func (m *memInner) DeleteObject(_ context.Context, _, key string) error {
	delete(m.objects, key)
	return nil
}
func (m *memInner) DeleteObjects(_ context.Context, _ string, keys []string) error {
	for _, k := range keys {
		delete(m.objects, k)
	}
	return nil
}
func (m *memInner) ObjectExists(_ context.Context, _, key string) (bool, error) {
	_, ok := m.objects[key]
	return ok, nil
}
func (m *memInner) GetObjectSize(_ context.Context, _, key string) (int64, error) {
	v, ok := m.objects[key]
	if !ok {
		return 0, errors.New("memInner: not found")
	}
	return int64(len(v)), nil
}
func (m *memInner) GetObjectChecksum(_ context.Context, _, key string) (string, error) {
	v, ok := m.objects[key]
	if !ok {
		return "", errors.New("memInner: not found")
	}
	sum := sha256.Sum256(v)
	return hex.EncodeToString(sum[:]), nil
}
func (m *memInner) GetPresignedUploadURL(_ context.Context, _, _ string, _ string, _ int64, _ string, _ time.Duration) (string, error) {
	return "", nil
}
func (m *memInner) GetPresignedDownloadURL(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	return "", nil
}

// fixedCipherLookup returns a CipherLookup that returns a stable
// per-drive cipher. The first lookup for a given driveID generates
// a new DEK and caches the cipher; subsequent lookups return the
// same cipher. This mirrors what a real implementation would do
// (a drive has one DEK, not one per call).
func fixedCipherLookup(master []byte) CipherLookup {
	provider, err := crypto.NewDEKProvider(master)
	if err != nil {
		panic(err)
	}
	cache := map[string]*crypto.NodeCipher{}
	return func(_ context.Context, driveID string) (*crypto.NodeCipher, error) {
		if c, ok := cache[driveID]; ok {
			return c, nil
		}
		wrapped, err := provider.NewWrappedDEK()
		if err != nil {
			return nil, err
		}
		dek, err := provider.Unwrap(wrapped)
		if err != nil {
			return nil, err
		}
		c, err := crypto.NewNodeCipher(dek)
		if err != nil {
			return nil, err
		}
		cache[driveID] = c
		return c, nil
	}
}

func TestRoundTripSmall(t *testing.T) {
	inner := newMemInner()
	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i)
	}
	store := New(inner, fixedCipherLookup(master))
	ctx := context.Background()

	plaintext := []byte("hello, world!")
	key := "drives/d1/uploads/u1"
	if err := store.PutObject(ctx, "b", key, bytes.NewReader(plaintext), int64(len(plaintext))); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	got, err := store.GetObject(ctx, "b", key)
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, plaintext)
	}
	stored := inner.objects[key]
	if bytes.Equal(stored, plaintext) {
		t.Fatalf("stored bytes are plaintext; expected ciphertext")
	}
}

func TestRoundTripMultiChunk(t *testing.T) {
	inner := newMemInner()
	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i)
	}
	store := New(inner, fixedCipherLookup(master))
	ctx := context.Background()

	plaintext := make([]byte, chunkSize*3+1234)
	for i := range plaintext {
		plaintext[i] = byte(i % 251)
	}
	key := "drives/d1/uploads/big"
	if err := store.PutObject(ctx, "b", key, bytes.NewReader(plaintext), int64(len(plaintext))); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	got, err := store.GetObject(ctx, "b", key)
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("multi-chunk round-trip mismatch: len(got)=%d, len(want)=%d", len(got), len(plaintext))
	}
}

func TestRoundTripEmpty(t *testing.T) {
	inner := newMemInner()
	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i)
	}
	store := New(inner, fixedCipherLookup(master))
	ctx := context.Background()

	key := "drives/d1/uploads/empty"
	if err := store.PutObject(ctx, "b", key, bytes.NewReader(nil), 0); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	got, err := store.GetObject(ctx, "b", key)
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("empty round-trip: got %d bytes, want 0", len(got))
	}
}

func TestAADMismatchOnKeySwap(t *testing.T) {
	inner := newMemInner()
	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i)
	}
	store := New(inner, fixedCipherLookup(master))
	ctx := context.Background()

	plaintext := []byte("important data")
	key1 := "drives/d1/uploads/u1"
	key2 := "drives/d1/uploads/u2"
	if err := store.PutObject(ctx, "b", key1, bytes.NewReader(plaintext), int64(len(plaintext))); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	ct := inner.objects[key1]
	inner.objects[key2] = ct
	if _, err := store.GetObject(ctx, "b", key2); err == nil {
		t.Fatalf("expected AAD-mismatch error on key swap, got nil")
	}
}

func TestMigratePlaintext(t *testing.T) {
	inner := newMemInner()
	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i)
	}
	store := New(inner, fixedCipherLookup(master))
	ctx := context.Background()

	plaintext := []byte("uploaded via presigned url")
	srcKey := "drives/d1/uploads/staging"
	dstKey := "drives/d1/uploads/final"
	if err := inner.PutObject(ctx, "b", srcKey, bytes.NewReader(plaintext), int64(len(plaintext))); err != nil {
		t.Fatalf("seed plaintext: %v", err)
	}
	if err := store.MigratePlaintext(ctx, "d1", "b", srcKey, dstKey); err != nil {
		t.Fatalf("MigratePlaintext: %v", err)
	}
	if _, ok := inner.objects[srcKey]; ok {
		t.Fatalf("plaintext object still present at srcKey")
	}
	if _, ok := inner.objects[dstKey]; !ok {
		t.Fatalf("ciphertext object missing at dstKey")
	}
	got, err := store.GetObject(ctx, "b", dstKey)
	if err != nil {
		t.Fatalf("GetObject(dstKey): %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("migrate round-trip: got %q, want %q", got, plaintext)
	}
}

func TestPassThroughWhenKeyHasNoDrivePrefix(t *testing.T) {
	inner := newMemInner()
	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i)
	}
	store := New(inner, fixedCipherLookup(master))
	ctx := context.Background()

	plaintext := []byte("legacy plaintext object")
	key := "some/legacy/key"
	if err := store.PutObject(ctx, "b", key, bytes.NewReader(plaintext), int64(len(plaintext))); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if !bytes.Equal(inner.objects[key], plaintext) {
		t.Fatalf("legacy object should be stored as plaintext (no drive prefix)")
	}
	got, err := store.GetObject(ctx, "b", key)
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("legacy GetObject mismatch")
	}
}

func TestNilLookupPassesThrough(t *testing.T) {
	inner := newMemInner()
	store := New(inner, nil)
	ctx := context.Background()

	plaintext := []byte("no encryption")
	key := "drives/d1/uploads/x"
	if err := store.PutObject(ctx, "b", key, bytes.NewReader(plaintext), int64(len(plaintext))); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if !bytes.Equal(inner.objects[key], plaintext) {
		t.Fatalf("nil lookup should pass through plaintext")
	}
}

func TestDriveIDFromKeyVariants(t *testing.T) {
	cases := []struct {
		key    string
		wantID string
		wantOK bool
	}{
		{"drives/d1/uploads/u1", "d1", true},
		{"drives/abc-123/files/f", "abc-123", true},
		{"some/legacy/key", "", false},
		{"drives", "", false},
		{"drives//uploads/u1", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		id, ok := driveIDFromKey(tc.key)
		if id != tc.wantID || ok != tc.wantOK {
			t.Errorf("driveIDFromKey(%q) = (%q, %v), want (%q, %v)", tc.key, id, ok, tc.wantID, tc.wantOK)
		}
	}
}

func TestTamperedCiphertextFails(t *testing.T) {
	inner := newMemInner()
	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i)
	}
	store := New(inner, fixedCipherLookup(master))
	ctx := context.Background()

	plaintext := []byte("integrity matters")
	key := "drives/d1/uploads/u1"
	if err := store.PutObject(ctx, "b", key, bytes.NewReader(plaintext), int64(len(plaintext))); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	ct := inner.objects[key]
	ct[nonceSize+sizeSize+1] ^= 0xFF
	inner.objects[key] = ct
	_, err := store.GetObject(ctx, "b", key)
	if err == nil {
		t.Fatalf("expected error on tampered ciphertext, got nil")
	}
	if !strings.Contains(err.Error(), "decrypt") {
		t.Fatalf("expected decrypt error, got: %v", err)
	}
}
