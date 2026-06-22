package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDEK(t *testing.T) []byte {
	t.Helper()
	dek := make([]byte, wrapKeySize)
	for i := range dek {
		dek[i] = byte(i + 1)
	}
	return dek
}

func TestWrapUnwrapRoundTrip(t *testing.T) {
	kek := newTestDEK(t)
	dek := bytes.Repeat([]byte{0xAB}, wrapKeySize)

	wrapped, err := Wrap(dek, kek)
	require.NoError(t, err)
	assert.NotEmpty(t, wrapped, "wrapped key should not be empty")

	got, err := Unwrap(wrapped, kek)
	require.NoError(t, err)
	assert.Equal(t, dek, got, "unwrapped DEK must equal the original")
}

func TestUnwrapWrongKey(t *testing.T) {
	dek := bytes.Repeat([]byte{0xCD}, wrapKeySize)
	kek := newTestDEK(t)
	other := bytes.Repeat([]byte{0xEE}, wrapKeySize)

	wrapped, err := Wrap(dek, kek)
	require.NoError(t, err)

	_, err = Unwrap(wrapped, other)
	assert.Error(t, err, "unwrapping with a different key must fail")
}

func TestUnwrapTamperedCiphertext(t *testing.T) {
	dek := bytes.Repeat([]byte{0x11}, wrapKeySize)
	kek := newTestDEK(t)

	wrapped, err := Wrap(dek, kek)
	require.NoError(t, err)

	raw, err := base64.StdEncoding.DecodeString(wrapped)
	require.NoError(t, err)
	// Flip a byte in the middle (the ciphertext, not the nonce).
	raw[len(raw)-5] ^= 0x01
	corrupted := base64.StdEncoding.EncodeToString(raw)

	_, err = Unwrap(corrupted, kek)
	assert.Error(t, err, "tampered ciphertext must fail to unwrap")
}

func TestWrapInvalidKeySize(t *testing.T) {
	_, err := Wrap([]byte("short"), make([]byte, 16))
	assert.Error(t, err)

	_, err = Unwrap("anything", make([]byte, 16))
	assert.Error(t, err)
}

func TestUnwrapMalformedBase64(t *testing.T) {
	_, err := Unwrap("!!!not-base64!!!", newTestDEK(t))
	assert.Error(t, err)
}

func TestUnwrapShortCiphertext(t *testing.T) {
	_, err := Unwrap(base64.StdEncoding.EncodeToString([]byte{1, 2, 3}), newTestDEK(t))
	assert.Error(t, err)
}

func TestNodeCipherRoundTrip(t *testing.T) {
	dek := newTestDEK(t)
	nc, err := NewNodeCipher(dek)
	require.NoError(t, err)

	driveID := "drive-A"
	nodeID := uuid.New()
	plaintext := []byte(`{"items":[{"name":"foo"}]}`)

	ct, err := nc.Encrypt(plaintext, driveID, nodeID)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ct, "ciphertext must differ from plaintext")
	assert.True(t, bytes.HasPrefix(ct, ct[:cGCMNonceSize()]),
		"ciphertext is expected to start with the nonce")

	pt, err := nc.Decrypt(ct, driveID, nodeID)
	require.NoError(t, err)
	assert.Equal(t, plaintext, pt)
}

func TestNodeCipherAADMismatch(t *testing.T) {
	dek := newTestDEK(t)
	nc, err := NewNodeCipher(dek)
	require.NoError(t, err)

	ct, err := nc.Encrypt([]byte("hello"), "drive-A", uuid.New())
	require.NoError(t, err)

	// Different driveID must fail.
	_, err = nc.Decrypt(ct, "drive-B", uuid.UUID{})
	assert.Error(t, err, "wrong driveID must fail")

	// Different nodeID must fail.
	_, err = nc.Decrypt(ct, "drive-A", uuid.New())
	assert.Error(t, err, "wrong nodeID must fail")
}

func TestNodeCipherRejectsOversize(t *testing.T) {
	dek := newTestDEK(t)
	nc, err := NewNodeCipher(dek)
	require.NoError(t, err)

	tooBig := make([]byte, nc.ContentCipherSize()+1)
	_, err = nc.Encrypt(tooBig, "drive-A", uuid.New())
	assert.Error(t, err)
}

// cGCMNonceSize returns the GCM nonce size used by the package. We
// can't reach cipher.AEAD from outside without storing the cipher
// alongside the test, so just hard-code 12 (the standard GCM size).
func cGCMNonceSize() int { return 12 }
