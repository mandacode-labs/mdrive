// Package session provides authentication session management backed by Valkey.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Session holds authenticated user context.
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Provider  string    `json:"provider"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// New creates a session with a random ID and the given TTL.
func New(ttl time.Duration) *Session {
	now := time.Now()
	return &Session{
		ID:        newID(),
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
}

// IsExpired reports whether the session has passed its ExpiresAt time.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// Encode serializes the session to JSON.
func (s *Session) Encode() ([]byte, error) {
	return json.Marshal(s)
}

// Decode deserializes a session from JSON.
func Decode(data []byte) (*Session, error) {
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("session: decode: %w", err)
	}
	return &s, nil
}

// TTL returns the remaining time until expiry.
func (s *Session) TTL() time.Duration {
	return time.Until(s.ExpiresAt)
}

func newID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("session: crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}
