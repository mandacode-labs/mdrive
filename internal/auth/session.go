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

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Session is the in-context view of the authenticated principal.
// The cookie payload is a separate (smaller) struct.
type Session struct {
	ID        string
	UserID    string
	Provider  string
	IsAdmin   bool
	CreatedAt time.Time
	ExpiresAt time.Time
}

// sessionData is the encrypted cookie payload. It carries the
// OIDC subject, resolved user id, and the raw id_token so logout
// can send `id_token_hint` to the IdP.
type sessionData struct {
	Subject   string    `json:"sub"`
	UserID    string    `json:"uid"`
	Provider  string    `json:"prv"`
	IsAdmin   bool      `json:"adm"`
	IDToken   string    `json:"idt"`
	Name      string    `json:"nm"`
	Email     string    `json:"em"`
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

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

func encrypt(plain []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", errorx.Wrap(err, fmt.Sprintf("aes: new cipher (key_len=%d)", len(key)))
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", errorx.Wrap(err, "aes: new gcm")
	}
	n := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, n); err != nil {
		return "", errorx.Wrap(err, "aes: nonce")
	}
	return base64.RawURLEncoding.EncodeToString(aesgcm.Seal(n, n, plain, nil)), nil
}

func decrypt(s string, key []byte) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, errorx.Wrap(err, "base64: decode")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("aes: new cipher (key_len=%d)", len(key)))
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errorx.Wrap(err, "aes: new gcm")
	}
	nonceSize := aesgcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, errorx.New(errorx.KindInvalidArgument, "aes: ciphertext too short (len="+itoa(len(raw))+", nonce_size="+itoa(nonceSize)+")")
	}
	plain, err := aesgcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return nil, errorx.Wrap(err, "aes: open")
	}
	return plain, nil
}

func (s *Service) setCookie(w http.ResponseWriter, r *http.Request, name, value string, maxAge int) {
	// CookieSecure comes from http.cookie.secure in config. Production
	// sits behind a TLS-terminating ingress, so r.TLS is always nil on
	// the pod -- checking it would set Secure=false even when the
	// operator has explicitly asked for Secure cookies, which makes
	// the browser drop them on cross-origin XHR from a subdomain.
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   s.cookieDomain,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: s.cookieSameSite,
	})
}

func (s *Service) writeSessionCookie(w http.ResponseWriter, r *http.Request, data *sessionData) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return errorx.Wrap(err, "session: marshal")
	}
	enc, err := encrypt(raw, s.encKey)
	if err != nil {
		return errorx.Wrap(err, "session: encrypt")
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
		return nil, errorx.New(errorx.KindInvalidArgument, "session: expired")
	}
	return &data, nil
}
