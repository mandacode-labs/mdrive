package session

import "context"

// MemoryStore is an in-memory Store for tests and development.
type MemoryStore struct {
	items map[string]*Session
}

// NewMemoryStore returns an in-memory store for tests.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: map[string]*Session{}}
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

var _ Store = (*MemoryStore)(nil)
