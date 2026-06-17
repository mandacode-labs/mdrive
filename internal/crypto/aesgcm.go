// Package crypto provides at-rest encryption helpers used by repositories
// to protect sensitive fields (e.g. S3 secret keys).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Cipher encrypts and decrypts small secrets at the repository boundary.
type Cipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// AESGCM implements Cipher with AES-256-GCM.
// Ciphertext format: base64(nonce(12) || ciphertext || gcmTag(16)).
type AESGCM struct {
	gcm cipher.AEAD
}

// NewAESGCM creates a Cipher from a 64-character hex-encoded 32-byte key.
func NewAESGCM(masterKeyHex string) (*AESGCM, error) {
	if len(masterKeyHex) != 64 {
		return nil, errors.New("crypto: master key must be 64 hex characters (32 bytes)")
	}
	key, err := hex.DecodeString(masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode master key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: create aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: create gcm: %w", err)
	}
	return &AESGCM{gcm: gcm}, nil
}

// GenerateMasterKey returns a fresh 64-character hex-encoded 32-byte key.
func GenerateMasterKey() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Encrypt encrypts plaintext and returns base64-encoded ciphertext.
func (a *AESGCM) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, a.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: generate nonce: %w", err)
	}
	out := a.gcm.Seal(nonce, nonce, plaintext, nil)
	return []byte(base64.StdEncoding.EncodeToString(out)), nil
}

// Decrypt decodes base64 ciphertext and returns plaintext.
func (a *AESGCM) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, errors.New("crypto: empty ciphertext")
	}
	data, err := base64.StdEncoding.DecodeString(string(ciphertext))
	if err != nil {
		return nil, fmt.Errorf("crypto: decode ciphertext: %w", err)
	}
	nonceSize := a.gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("crypto: ciphertext too short")
	}
	nonce, ct := data[:nonceSize], data[nonceSize:]
	return a.gcm.Open(nil, nonce, ct, nil)
}

// NoOp is a Cipher that does nothing. Useful for tests.
type NoOp struct{}

func (NoOp) Encrypt(p []byte) ([]byte, error) { return p, nil }
func (NoOp) Decrypt(c []byte) ([]byte, error) { return c, nil }

// ConstantTimeEq compares two strings in constant time.
func ConstantTimeEq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
