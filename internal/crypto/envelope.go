package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

const (
	wrapKeySize   = 32
	wrapNonceSize = 12
)

// Wrap encrypts dek (a 32-byte data encryption key) with kek (a 32-byte
// key-encryption key) and returns a base64 string safe for DB storage.
// Wire format: base64( nonce(12) || aesgcm(plaintext) || tag(16) ).
// Key wrapping is independent of context; callers that need
// context binding should derive a context-specific KEK.
func Wrap(dek, kek []byte) (string, error) {
	if len(kek) != wrapKeySize {
		return "", fmt.Errorf("crypto: KEK must be %d bytes, got %d", wrapKeySize, len(kek))
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return "", fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: new gcm: %w", err)
	}
	nonce := make([]byte, wrapNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: generate nonce: %w", err)
	}
	ct := gcm.Seal(nonce, nonce, dek, nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Unwrap is the inverse of Wrap.
func Unwrap(wrapped string, kek []byte) ([]byte, error) {
	if len(kek) != wrapKeySize {
		return nil, fmt.Errorf("crypto: KEK must be %d bytes, got %d", wrapKeySize, len(kek))
	}
	data, err := base64.StdEncoding.DecodeString(wrapped)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode wrapped key: %w", err)
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	if len(data) < wrapNonceSize+gcm.Overhead() {
		return nil, fmt.Errorf("crypto: wrapped key too short")
	}
	nonce, ct := data[:wrapNonceSize], data[wrapNonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

// DEKProvider generates per-drive data encryption keys, wrapping them
// with a 32-byte key-encryption key (the master key from config).
type DEKProvider struct {
	kek []byte
}

// NewDEKProvider returns a DEKProvider keyed by kek. kek must be 32 bytes.
func NewDEKProvider(kek []byte) (*DEKProvider, error) {
	if len(kek) != wrapKeySize {
		return nil, fmt.Errorf("crypto: DEKProvider requires %d-byte key, got %d", wrapKeySize, len(kek))
	}
	return &DEKProvider{kek: append([]byte(nil), kek...)}, nil
}

// NewWrappedDEK generates a fresh 32-byte DEK and returns it wrapped
// with the KEK.
func (p *DEKProvider) NewWrappedDEK() (string, error) {
	dek := make([]byte, wrapKeySize)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return "", fmt.Errorf("crypto: generate DEK: %w", err)
	}
	return Wrap(dek, p.kek)
}
