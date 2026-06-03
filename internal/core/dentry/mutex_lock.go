package dentry

import (
	"context"
	"fmt"
	"sync"
)

// Locker provides directory-level serialization for concurrent operations.
// Uses in-memory mutexes keyed by directory ID.
type Locker struct {
	mu sync.Map // map[string]*sync.Mutex
}

// NewLocker creates a new directory locker.
func NewLocker() *Locker {
	return &Locker{}
}

// Lock acquires an exclusive lock for the given directory ID.
// Returns an unlock function that must be called when done.
func (l *Locker) Lock(dirID string) func() {
	mu, _ := l.mu.LoadOrStore(dirID, &sync.Mutex{})
	m := mu.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

// LockContext acquires a lock for the given directory ID with context support.
// Returns an error if the context is cancelled while waiting for the lock.
func (l *Locker) LockContext(ctx context.Context, dirID string) (func(), error) {
	mu, _ := l.mu.LoadOrStore(dirID, &sync.Mutex{})
	m := mu.(*sync.Mutex)

	done := make(chan struct{})
	go func() {
		m.Lock()
		close(done)
	}()

	select {
	case <-done:
		return m.Unlock, nil
	case <-ctx.Done():
		go func() {
			<-done
			m.Unlock()
		}()
		return nil, fmt.Errorf("context cancelled while waiting for lock on directory %s", dirID)
	}
}
