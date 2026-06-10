package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/mandacode-labs/mdrive/internal/errors"
)

// SessionID is the type for session identifiers.
type SessionID string

// Session represents a user session.
type Session struct {
	id        SessionID
	userID    string
	expiresAt time.Time
	createdAt time.Time
	idToken   string
}

// NewSession creates a new session.
func NewSession(id SessionID, userID string, expiresAt, createdAt time.Time, idToken string) *Session {
	return &Session{
		id:        id,
		userID:    userID,
		expiresAt: expiresAt,
		createdAt: createdAt,
		idToken:   idToken,
	}
}

// ID returns the session ID.
func (s *Session) ID() SessionID {
	return s.id
}

// UserID returns the user ID.
func (s *Session) UserID() string {
	return s.userID
}

// ExpiresAt returns the expiration time.
func (s *Session) ExpiresAt() time.Time {
	return s.expiresAt
}

// CreatedAt returns the creation time.
func (s *Session) CreatedAt() time.Time {
	return s.createdAt
}

// IDToken returns the stored OIDC ID token.
func (s *Session) IDToken() string {
	return s.idToken
}

// IsExpired checks if the session is expired.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.expiresAt)
}

// SessionService defines the session service interface.
type SessionService interface {
	// Create creates a new session for a user.
	Create(ctx context.Context, userID string, idToken string) (*Session, error)

	// Get retrieves a session by ID.
	Get(ctx context.Context, id SessionID) (*Session, error)

	// Validate validates if session is still valid.
	Validate(ctx context.Context, id SessionID) (*Session, error)

	// Delete deletes a session (logout).
	Delete(ctx context.Context, id SessionID) error

	// DeleteByUserID deletes all sessions for a user.
	DeleteByUserID(ctx context.Context, userID string) error
}

type sessionService struct {
	repo SessionRepository
	ttl  time.Duration
}

// NewSessionService creates a new session service.
func NewSessionService(repo SessionRepository, ttl time.Duration) SessionService {
	return &sessionService{
		repo: repo,
		ttl:  ttl,
	}
}

// Create creates a new session for a user.
func (s *sessionService) Create(ctx context.Context, userID string, idToken string) (*Session, error) {
	now := time.Now()
	id := SessionID(generateSessionID())
	session := NewSession(id, userID, now.Add(s.ttl), now, idToken)

	if err := s.repo.Save(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

// Get retrieves a session by ID.
func (s *sessionService) Get(ctx context.Context, id SessionID) (*Session, error) {
	return s.repo.Get(ctx, id)
}

// Validate validates if session is still valid.
func (s *sessionService) Validate(ctx context.Context, id SessionID) (*Session, error) {
	session, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, errors.NotFound("session not found")
	}
	if session.IsExpired() {
		return nil, errors.Unauthorized("session expired")
	}
	return session, nil
}

// Delete deletes a session (logout).
func (s *sessionService) Delete(ctx context.Context, id SessionID) error {
	return s.repo.Delete(ctx, id)
}

// DeleteByUserID deletes all sessions for a user.
func (s *sessionService) DeleteByUserID(ctx context.Context, userID string) error {
	return s.repo.DeleteByUserID(ctx, userID)
}

// generateSessionID generates a random session ID.
func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))[:32]
	}
	return hex.EncodeToString(b)
}
