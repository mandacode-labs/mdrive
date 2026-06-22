// Package crypto provides at-rest encryption primitives used across
// the storage layer: AES-256-GCM for small secrets (e.g. S3 secret
// keys, per-drive data encryption keys) and a content cipher that
// authenticates (driveID, nodeID) as additional data to prevent
// ciphertext swap between nodes.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/google/uuid"
)

// wrapCiphertext is the wire format for a wrapped key:
//
//	base64( nonce(12) || aesgcm(plaintext, nonce, aad) )
//
// where aad is empty (key wrapping is independent of context) and
// the implicit AAD size from the GCM tag is 16 bytes.
const (
	wrapNonceSize = 12
	wrapKeySize   = 32 // AES-256
)

// wrapKey builds the cipher used for key wrapping. The KEK must be
// 32 bytes (AES-256). The returned AEAD reuses a fresh random nonce
// for every wrap; the caller is responsible for providing a distinct
// (nonce, ciphertext) pair per wrap call.
func wrapKey(kek []byte) (cipher.AEAD, error) {
	if len(kek) != wrapKeySize {
		return nil, fmt.Errorf("crypto: wrap key must be %d bytes, got %d", wrapKeySize, len(kek))
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	return cipher.NewGCM(block)
}

// Wrap encrypts dek (a 32-byte data encryption key) with kek (a
// 32-byte key-encryption key) and returns a base64 string safe for
// DB storage. The returned ciphertext is bound to no additional
// data; callers that need context binding should derive a
// context-specific KEK rather than rely on GCM AAD.
func Wrap(dek, kek []byte) (string, error) {
	gcm, err := wrapKey(kek)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, wrapNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: generate nonce: %w", err)
	}
	ct := gcm.Seal(nonce, nonce, dek, nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Unwrap is the inverse of Wrap: it decodes the base64 ciphertext
// and decrypts it with kek, returning the original 32-byte DEK.
// Authentication failures (wrong kek, tampered ciphertext) return
// an error.
func Unwrap(wrapped string, kek []byte) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(wrapped)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode wrapped key: %w", err)
	}
	gcm, err := wrapKey(kek)
	if err != nil {
		return nil, err
	}
	if len(data) < wrapNonceSize+gcm.Overhead() {
		return nil, fmt.Errorf("crypto: wrapped key too short")
	}
	nonce, ct := data[:wrapNonceSize], data[wrapNonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

// contentCipherSize caps the plaintext size for NodeCipher. The
// existing node content limit is 4 KiB; this constant is kept in
// sync via the node package's MaxContentSize (see core/node).
const contentCipherSize = 4 * 1024

// NodeCipher encrypts and decrypts small node content blobs
// (typically the JSON-serialized content stored inline on a node
// row) using a per-drive data encryption key. Each Encrypt/Decrypt
// call binds the ciphertext to (driveID, nodeID) as AES-GCM
// additional data, so a ciphertext recorded against one node cannot
// be replayed against another even by a privileged DB reader.
type NodeCipher struct {
	gcm cipher.AEAD
}

// NewNodeCipher returns a NodeCipher that uses dek (32 bytes) as its
// key.
func NewNodeCipher(dek []byte) (*NodeCipher, error) {
	gcm, err := wrapKey(dek)
	if err != nil {
		return nil, err
	}
	return &NodeCipher{gcm: gcm}, nil
}

// contentAAD is the additional data bound to every node-content
// ciphertext. Encoding: driveID || ":" || nodeID. The driveID and
// nodeID both come from the database; encoding them as AAD prevents
// an attacker from swapping ciphertexts between rows.
func contentAAD(driveID string, nodeID uuid.UUID) []byte {
	out := make([]byte, 0, len(driveID)+1+36)
	out = append(out, driveID...)
	out = append(out, ':')
	out = append(out, nodeID.String()...)
	return out
}

// Encrypt seals plaintext with the DEK and the (driveID, nodeID)
// AAD. The output is the wire format that goes into the DB
// `content` column.
func (c *NodeCipher) Encrypt(plaintext []byte, driveID string, nodeID uuid.UUID) ([]byte, error) {
	if len(plaintext) > contentCipherSize {
		return nil, fmt.Errorf("crypto: node content exceeds %d bytes", contentCipherSize)
	}
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: generate nonce: %w", err)
	}
	return c.gcm.Seal(nonce, nonce, plaintext, contentAAD(driveID, nodeID)), nil
}

// Decrypt opens a ciphertext produced by Encrypt. Authentication
// failures (wrong DEK, tampered ciphertext, AAD mismatch) return an
// error; the plaintext is only returned when the AAD matches.
func (c *NodeCipher) Decrypt(ciphertext []byte, driveID string, nodeID uuid.UUID) ([]byte, error) {
	if len(ciphertext) < c.gcm.NonceSize()+c.gcm.Overhead() {
		return nil, fmt.Errorf("crypto: ciphertext too short")
	}
	nonce, ct := ciphertext[:c.gcm.NonceSize()], ciphertext[c.gcm.NonceSize():]
	return c.gcm.Open(nil, nonce, ct, contentAAD(driveID, nodeID))
}

// ContentCipherSize returns the maximum plaintext size accepted by
// Encrypt. Exposed so callers can validate before invoking.
func (c *NodeCipher) ContentCipherSize() int { return contentCipherSize }
