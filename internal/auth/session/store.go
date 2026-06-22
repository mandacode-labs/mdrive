package session

import (
	"context"
	"errors"
)

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

// Scanner is an optional capability of a Store. Backends that support
// iteration (e.g. Valkey) implement it so the GC worker can find expired
// sessions. Backends that do not can omit it; the GC will log and skip.
type Scanner interface {
	Scan(ctx context.Context, fn func(id string) error) error
}
