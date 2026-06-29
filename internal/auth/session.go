package auth

import "time"

type Session struct {
	ID        string
	UserID    string
	Provider  string
	IsAdmin   bool
	CreatedAt time.Time
	ExpiresAt time.Time
}

func (s *Session) IsExpired() bool {
	if s == nil {
		return true
	}
	return !s.ExpiresAt.IsZero() && time.Now().After(s.ExpiresAt)
}