package upload

import (
	"context"
	"time"
)

// TokenRegistry stores and retrieves upload tokens with a TTL.
type TokenRegistry interface {
	Put(ctx context.Context, meta PresignMeta, ttl time.Duration) error
	Get(ctx context.Context, uploadID string) (PresignMeta, error)
	Delete(ctx context.Context, uploadID string) error
}

// TokenScanner is an optional capability of a TokenRegistry.
// Backends that support iteration (e.g. Valkey/Redis) implement
// it so the GC worker can find expired tokens to clean up.
// Backends that do not (e.g. in-memory) can omit it; the GC will
// log and skip.
type TokenScanner interface {
	Scan(ctx context.Context, fn func(id string) error) error
}
