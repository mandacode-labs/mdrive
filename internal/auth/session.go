package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Session struct {
	ID        string
	UserID    string
	Provider  string
	IsAdmin   bool
	CreatedAt time.Time
	ExpiresAt time.Time
}

type sessionData struct {
	Subject   string    `json:"sub"`
	UserID    string    `json:"uid"`
	Provider  string    `json:"prv"`
	IsAdmin   bool      `json:"adm"`
	ExpiresAt time.Time `json:"exp"`
}

func (s *sessionData) IsExpired() bool {
	return !s.ExpiresAt.IsZero() && time.Now().After(s.ExpiresAt)
}

func nonce() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func encrypt(plain []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes: new cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("aes: new gcm: %w", err)
	}
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("aes: nonce: %w", err)
	}
	ciphertext := aesgcm.Seal(nil, nonce, plain, nil)
	return base64.RawURLEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func decrypt(s string, key []byte) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("base64: decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: new cipher: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes: new gcm: %w", err)
	}
	nonceSize := aesgcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, fmt.Errorf("aes: ciphertext too short")
	}
	plain, err := aesgcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("aes: open: %w", err)
	}
	return plain, nil
}

func (s *Service) setCookie(w http.ResponseWriter, r *http.Request, name, value string, maxAge int) {
	secure := r.TLS != nil
	http.SetCookie(w, &http.Cookie{ // #nosec G124 - Secure dynamically set based on TLS
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   s.cookieDomain,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: s.cookieSameSite,
	})
}

func (s *Service) writeSessionCookie(w http.ResponseWriter, r *http.Request, data *sessionData) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("session: marshal: %w", err)
	}
	enc, err := encrypt(raw, s.encKey)
	if err != nil {
		return fmt.Errorf("session: encrypt: %w", err)
	}
	maxAge := int(time.Until(data.ExpiresAt).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	s.setCookie(w, r, s.cookieName, enc, maxAge)
	return nil
}

func (s *Service) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	s.setCookie(w, r, s.cookieName, "", -1)
}

func (s *Service) readSessionCookie(r *http.Request) (*sessionData, error) {
	c, err := r.Cookie(s.cookieName)
	if err != nil {
		return nil, err
	}
	raw, err := decrypt(c.Value, s.encKey)
	if err != nil {
		return nil, err
	}
	var data sessionData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data.IsExpired() {
		return nil, fmt.Errorf("session: expired")
	}
	return &data, nil
}
