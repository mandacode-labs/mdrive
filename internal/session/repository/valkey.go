package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	domain "github.com/mandacode-labs/mdrive/internal/session"
	"github.com/valkey-io/valkey-go"
)

// sessionData is the serializable representation of Session for Valkey storage.
type sessionData struct {
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	IDToken   string    `json:"id_token,omitempty"`
}

// toSessionData converts Session to sessionData.
func toSessionData(s *domain.Session) *sessionData {
	return &sessionData{
		UserID:    s.UserID(),
		ExpiresAt: s.ExpiresAt(),
		CreatedAt: s.CreatedAt(),
		IDToken:   s.IDToken(),
	}
}

// ValkeySessionRepository implements SessionRepository using Valkey.
type ValkeySessionRepository struct {
	client valkey.Client
	prefix string
}

// NewValkeySessionRepository creates a new ValkeySessionRepository.
func NewValkeySessionRepository(client valkey.Client, prefix string) domain.SessionRepository {
	return &ValkeySessionRepository{
		client: client,
		prefix: prefix,
	}
}

func (r *ValkeySessionRepository) sessionKey(id domain.SessionID) string {
	return fmt.Sprintf("%s:session:%s", r.prefix, id)
}

func (r *ValkeySessionRepository) userSessionsKey(userID string) string {
	return fmt.Sprintf("%s:user:sessions:%s", r.prefix, userID)
}

// Save saves a session.
func (r *ValkeySessionRepository) Save(ctx context.Context, session *domain.Session) error {
	data, err := json.Marshal(toSessionData(session))
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	ttl := time.Until(session.ExpiresAt())
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	err = r.client.Do(ctx, r.client.B().Set().
		Key(r.sessionKey(session.ID())).
		Value(string(data)).
		ExSeconds(int64(ttl.Seconds())).
		Build()).Error()
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	err = r.client.Do(ctx, r.client.B().Sadd().
		Key(r.userSessionsKey(session.UserID())).
		Member(string(session.ID())).
		Build()).Error()
	if err != nil {
		return fmt.Errorf("add to user sessions: %w", err)
	}

	return nil
}

// Get retrieves a session by ID.
func (r *ValkeySessionRepository) Get(ctx context.Context, id domain.SessionID) (*domain.Session, error) {
	result := r.client.Do(ctx, r.client.B().Get().Key(r.sessionKey(id)).Build())
	if result.Error() != nil {
		return nil, nil
	}

	data, err := result.ToString()
	if err != nil {
		return nil, nil
	}

	var sd sessionData
	if err := json.Unmarshal([]byte(data), &sd); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return domain.NewSession(id, sd.UserID, sd.ExpiresAt, sd.CreatedAt, sd.IDToken), nil
}

// Delete deletes a session by ID.
func (r *ValkeySessionRepository) Delete(ctx context.Context, id domain.SessionID) error {
	s, _ := r.Get(ctx, id)

	if s != nil {
		_ = r.client.Do(ctx, r.client.B().Srem().
			Key(r.userSessionsKey(s.UserID())).
			Member(string(id)).
			Build()).Error()
	}

	delErr := r.client.Do(ctx, r.client.B().Del().Key(r.sessionKey(id)).Build()).Error()
	if delErr != nil {
		return fmt.Errorf("delete session: %w", delErr)
	}
	return nil
}

// DeleteByUserID deletes all sessions for a user.
func (r *ValkeySessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	result := r.client.Do(ctx, r.client.B().Smembers().Key(r.userSessionsKey(userID)).Build())
	if result.Error() != nil {
		return fmt.Errorf("get user sessions: %w", result.Error())
	}

	members, err := result.AsStrSlice()
	if err != nil {
		return fmt.Errorf("parse user sessions: %w", err)
	}

	if len(members) > 0 {
		keys := make([]string, 0, len(members))
		for _, sid := range members {
			keys = append(keys, r.sessionKey(domain.SessionID(sid)))
		}

		if err := r.client.Do(ctx, r.client.B().Del().Key(keys...).Build()).Error(); err != nil {
			return fmt.Errorf("delete sessions: %w", err)
		}
	}

	if err := r.client.Do(ctx, r.client.B().Del().Key(r.userSessionsKey(userID)).Build()).Error(); err != nil {
		return fmt.Errorf("delete user sessions set: %w", err)
	}
	return nil
}
