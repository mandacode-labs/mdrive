package crypto

import (
	"encoding/hex"
	"testing"
)

func TestAESGCMRoundTrip(t *testing.T) {
	key, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	c, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	plaintext := []byte("my-s3-secret-key-123")
	ct, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if string(ct) == string(plaintext) {
		t.Error("ciphertext should not equal plaintext")
	}
	got, err := c.Decrypt(ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Errorf("expected %q, got %q", plaintext, got)
	}
}

func TestAESGCMWrongKey(t *testing.T) {
	key1, _ := GenerateMasterKey()
	key2, _ := GenerateMasterKey()
	c1, _ := NewAESGCM(key1)
	c2, _ := NewAESGCM(key2)
	ct, _ := c1.Encrypt([]byte("secret"))
	if _, err := c2.Decrypt(ct); err == nil {
		t.Error("expected decryption failure with wrong key")
	}
}

func TestAESGCMTampered(t *testing.T) {
	key, _ := GenerateMasterKey()
	c, _ := NewAESGCM(key)
	ct, _ := c.Encrypt([]byte("secret"))
	ct[len(ct)-1] ^= 0xFF
	if _, err := c.Decrypt(ct); err == nil {
		t.Error("expected decryption failure with tampered ciphertext")
	}
}

func TestAESGCMInvalidKeyLength(t *testing.T) {
	if _, err := NewAESGCM(hex.EncodeToString([]byte("short"))); err == nil {
		t.Error("expected error for short key")
	}
}

func TestNoOp(t *testing.T) {
	var c NoOp
	p := []byte("plain")
	ct, err := c.Encrypt(p)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	got, err := c.Decrypt(ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(got) != string(p) {
		t.Errorf("no-op mismatch")
	}
}
