package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
)

// aesGCM is the standard AES-GCM (authenticated encryption)
// implementation. Output format: nonce (12) || ciphertext.
type aesGCM struct {
	gcm cipher.AEAD
}

// NewFromEnvKey creates a crypto pair from the MD_CRYPT_KEY
// env var (32 bytes raw). Returns both Encryptor and
// Decryptor (they share the same AEAD instance).
func NewFromEnvKey() (Encryptor, Decryptor, error) {
	keyStr := os.Getenv("MD_CRYPT_KEY")
	if keyStr == "" {
		return nil, nil, errors.New("crypto: MD_CRYPT_KEY env var is required (32 bytes)")
	}
	if len(keyStr) < 32 {
		return nil, nil, fmt.Errorf("crypto: MD_CRYPT_KEY must be at least 32 bytes (got %d)", len(keyStr))
	}
	key := []byte(keyStr[:32])

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	a := &aesGCM{gcm: gcm}
	return a, a, nil
}

// Encrypt seals plaintext with a random nonce.
func (a *aesGCM) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, a.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: read nonce: %w", err)
	}
	// Seal appends ciphertext+tag to nonce; result is
	// nonce || ciphertext || tag.
	return a.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt opens a nonce-prefixed ciphertext.
func (a *aesGCM) Decrypt(ciphertext []byte) ([]byte, error) {
	ns := a.gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, errors.New("crypto: ciphertext too short")
	}
	nonce, ct := ciphertext[:ns], ciphertext[ns:]
	return a.gcm.Open(nil, nonce, ct, nil)
}