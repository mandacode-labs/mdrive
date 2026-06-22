package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"

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

func TestDEKProviderNewWrappedDEK(t *testing.T) {
	master := newTestDEK(t)
	provider, err := NewDEKProvider(master)
	require.NoError(t, err)

	wrapped, err := provider.NewWrappedDEK()
	require.NoError(t, err)
	assert.NotEmpty(t, wrapped)

	got, err := Unwrap(wrapped, master)
	require.NoError(t, err)
	assert.Len(t, got, wrapKeySize)
}

func TestDEKProviderDistinctKeys(t *testing.T) {
	master := newTestDEK(t)
	provider, err := NewDEKProvider(master)
	require.NoError(t, err)

	a, err := provider.NewWrappedDEK()
	require.NoError(t, err)
	b, err := provider.NewWrappedDEK()
	require.NoError(t, err)
	assert.NotEqual(t, a, b, "two NewWrappedDEK calls must produce distinct DEKs")
}

func TestDEKProviderInvalidKeySize(t *testing.T) {
	_, err := NewDEKProvider(make([]byte, 16))
	assert.Error(t, err)
}
