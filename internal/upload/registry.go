package upload

import (
	"context"
	"time"
)

// Registry stores and retrieves upload tokens with a TTL.
type Registry interface {
	Put(ctx context.Context, meta PresignMeta, ttl time.Duration) error
	Get(ctx context.Context, uploadID string) (PresignMeta, error)
	Delete(ctx context.Context, uploadID string) error
}

// Scanner is an optional capability of a Registry. Backends that
// support iteration (e.g. Valkey/Redis) implement it so the GC
// worker can find expired tokens to clean up. Backends that do
// not (e.g. in-memory) can omit it; the GC will log and skip.
type Scanner interface {
	Scan(ctx context.Context, fn func(id string) error) error
}
