// Package crypto provides authenticated symmetric encryption
// for sensitive fields (e.g. storage secret keys). The key
// comes from the MD_CRYPT_KEY env var (32 bytes). Output is
// self-contained: nonce || ciphertext || tag.
package crypto

// Encryptor encrypts plaintext into ciphertext.
type Encryptor interface {
	Encrypt(plaintext []byte) ([]byte, error)
}

// Decryptor decrypts ciphertext into plaintext.
type Decryptor interface {
	Decrypt(ciphertext []byte) ([]byte, error)
}