package upload

import (
	"context"
	"sync"
	"time"
)

// MemoryRegistry is an in-memory Registry implementation for tests and single-instance deployments.
type MemoryRegistry struct {
	mu    sync.RWMutex
	items map[string]item
}

type item struct {
	meta PresignMeta
	exp  time.Time
}

// NewMemoryRegistry creates a new in-memory upload registry.
func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{items: map[string]item{}}
}

func (r *MemoryRegistry) Put(ctx context.Context, meta PresignMeta, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	r.items[meta.UploadID] = item{meta: meta, exp: time.Now().Add(ttl)}
	return nil
}

func (r *MemoryRegistry) Get(ctx context.Context, uploadID string) (PresignMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	select {
	case <-ctx.Done():
		return PresignMeta{}, ctx.Err()
	default:
	}
	it, ok := r.items[uploadID]
	if !ok || time.Now().After(it.exp) {
		return PresignMeta{}, ErrNotFound
	}
	return it.meta, nil
}

func (r *MemoryRegistry) Delete(ctx context.Context, uploadID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	delete(r.items, uploadID)
	return nil
}
