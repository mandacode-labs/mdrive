package vfs

import (
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// countingRepo is a node.Repository that records how many times
// GetByID is called per UUID. It lets the resolver cache test
// verify that repeat loads are short-circuited by the cache.
type countingRepo struct {
	mu    sync.Mutex
	calls map[uuid.UUID]int
	byID  map[uuid.UUID]*node.Node
}

func newCountingRepo() *countingRepo {
	return &countingRepo{calls: map[uuid.UUID]int{}, byID: map[uuid.UUID]*node.Node{}}
}

func (c *countingRepo) seed(n *node.Node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byID[n.ID()] = n
}

func (c *countingRepo) count(id uuid.UUID) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls[id]
}

func (c *countingRepo) Get(_ context.Context, id uuid.UUID) (*node.Node, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls[id]++
	n, ok := c.byID[id]
	if !ok {
		return nil, errorx.New(errorx.KindNotFound, "")
	}
	return n, nil
}

func (c *countingRepo) Save(_ context.Context, n *node.Node) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byID[n.ID()] = n
	return nil
}

func (c *countingRepo) Delete(_ context.Context, id uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.byID, id)
	return nil
}

func (c *countingRepo) WithTx(_ context.Context, fn func(node.Repository) error) error {
	return fn(c)
}

func TestResolverCachesGetByID(t *testing.T) {
	repo := newCountingRepo()
	d, err := node.NewDirectory()
	require.NoError(t, err)
	repo.seed(d)

	r := newResolver(node.NewService(repo))

	first, err := r.loadByID(context.Background(), d.ID())
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := r.loadByID(context.Background(), d.ID())
	require.NoError(t, err)
	assert.Same(t, first, second, "cache must return the same pointer for the same UUID within a single resolver")
	assert.Equal(t, 1, repo.count(d.ID()), "the second load must hit the cache")
}

func TestResolverInstancesAreIndependent(t *testing.T) {
	repo := newCountingRepo()
	d, err := node.NewDirectory()
	require.NoError(t, err)
	repo.seed(d)

	r1 := newResolver(node.NewService(repo))
	r2 := newResolver(node.NewService(repo))

	_, err = r1.loadByID(context.Background(), d.ID())
	require.NoError(t, err)
	_, err = r2.loadByID(context.Background(), d.ID())
	require.NoError(t, err)

	assert.Equal(t, 2, repo.count(d.ID()), "two independent resolvers must each hit the NodeClient")
}
