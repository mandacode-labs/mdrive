package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
)

// Common errors.
var (
	ErrNotFound = errors.New("session: not found")
	ErrExpired  = errors.New("session: expired")
)

// Store persists and retrieves authentication sessions.
type Store interface {
	Create(ctx context.Context, s *Session) error
	Get(ctx context.Context, id string) (*Session, error)
	Delete(ctx context.Context, id string) error
}

// ValkeyStore implements Store with Valkey.
type ValkeyStore struct {
	client valkey.Client
	prefix string
}

// NewValkeyStore creates a session store backed by Valkey.
func NewValkeyStore(client valkey.Client) *ValkeyStore {
	return &ValkeyStore{client: client, prefix: "mdrive:session:"}
}

func (s *ValkeyStore) key(id string) string {
	return s.prefix + id
}

// Create saves a session with the session's remaining TTL.
func (s *ValkeyStore) Create(ctx context.Context, sess *Session) error {
	data, err := sess.Encode()
	if err != nil {
		return err
	}
	ttl := sess.TTL()
	if ttl <= 0 {
		return ErrExpired
	}
	resp := s.client.Do(ctx, s.client.B().Set().Key(s.key(sess.ID)).Value(valkey.BinaryString(data)).ExSeconds(int64(ttl.Seconds())).Build())
	if err := resp.Error(); err != nil {
		return fmt.Errorf("session: valkey set: %w", err)
	}
	return nil
}

// Get retrieves and validates a session.
func (s *ValkeyStore) Get(ctx context.Context, id string) (*Session, error) {
	resp := s.client.Do(ctx, s.client.B().Get().Key(s.key(id)).Build())
	if err := resp.Error(); err != nil {
		if errors.Is(err, valkey.Nil) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("session: valkey get: %w", err)
	}
	data, err := resp.AsBytes()
	if err != nil {
		return nil, fmt.Errorf("session: valkey as bytes: %w", err)
	}
	sess, err := Decode(data)
	if err != nil {
		return nil, err
	}
	if sess.IsExpired() {
		_ = s.Delete(ctx, id)
		return nil, ErrExpired
	}
	return sess, nil
}

// Delete removes a session.
func (s *ValkeyStore) Delete(ctx context.Context, id string) error {
	resp := s.client.Do(ctx, s.client.B().Del().Key(s.key(id)).Build())
	if err := resp.Error(); err != nil {
		return fmt.Errorf("session: valkey del: %w", err)
	}
	return nil
}

// NewMemoryStore returns an in-memory store for tests.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: map[string]*Session{}}
}

// MemoryStore is an in-memory Store for tests.
type MemoryStore struct {
	items map[string]*Session
}

func (s *MemoryStore) Create(_ context.Context, sess *Session) error {
	s.items[sess.ID] = sess
	return nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (*Session, error) {
	sess, ok := s.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	if sess.IsExpired() {
		delete(s.items, id)
		return nil, ErrExpired
	}
	return sess, nil
}

func (s *MemoryStore) Delete(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}

// Compile-time check.
var (
	_ Store = (*ValkeyStore)(nil)
	_ Store = (*MemoryStore)(nil)
	_       = time.Now
)
