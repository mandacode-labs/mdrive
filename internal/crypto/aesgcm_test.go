package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAESGCMRoundTrip(t *testing.T) {
	key, err := GenerateMasterKey()
	require.NoError(t, err)
	c, err := NewAESGCM(key)
	require.NoError(t, err)

	plaintext := []byte("my-s3-secret-key-123")
	ct, err := c.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ct, "ciphertext should not equal plaintext")

	got, err := c.Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestAESGCMWrongKey(t *testing.T) {
	key1, _ := GenerateMasterKey()
	key2, _ := GenerateMasterKey()
	c1, _ := NewAESGCM(key1)
	c2, _ := NewAESGCM(key2)
	ct, _ := c1.Encrypt([]byte("secret"))
	_, err := c2.Decrypt(ct)
	assert.Error(t, err, "expected decryption failure with wrong key")
}

func TestAESGCMTampered(t *testing.T) {
	key, _ := GenerateMasterKey()
	c, _ := NewAESGCM(key)
	ct, _ := c.Encrypt([]byte("secret"))
	ct[len(ct)-1] ^= 0xFF
	_, err := c.Decrypt(ct)
	assert.Error(t, err, "expected decryption failure with tampered ciphertext")
}

func TestAESGCMInvalidKeyLength(t *testing.T) {
	_, err := NewAESGCM(hex.EncodeToString([]byte("short")))
	assert.Error(t, err)
}

func TestNoOp(t *testing.T) {
	var c NoOp
	p := []byte("plain")
	ct, err := c.Encrypt(p)
	require.NoError(t, err)
	got, err := c.Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, p, got)
}