package node

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo is a minimal in-memory Repository for testing nlink logic.
type fakeRepo struct {
	nodes   map[uuid.UUID]*Node
	deleted map[uuid.UUID]bool
	saveErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{nodes: map[uuid.UUID]*Node{}, deleted: map[uuid.UUID]bool{}}
}

func (r *fakeRepo) Get(_ context.Context, id uuid.UUID) (*Node, error) {
	if r.deleted[id] {
		return nil, ErrNotFound
	}
	n, ok := r.nodes[id]
	if !ok {
		return nil, ErrNotFound
	}
	return n, nil
}

func (r *fakeRepo) Save(_ context.Context, n *Node) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.nodes[n.id] = n
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.deleted[id] = true
	delete(r.nodes, id)
	return nil
}

func (r *fakeRepo) WithTx(_ context.Context, fn func(Repository) error) error {
	return fn(r)
}

func TestLinkIncrementsNLink(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	child, err := NewFile("hello")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), child))
	require.Equal(t, uint32(0), child.NLink(), "freshly created node has nlink=0 (POSIX)")

	require.NoError(t, svc.Link(context.Background(), dir, "a", child))
	assert.Equal(t, uint32(1), child.NLink(), "first Link should set nlink=1")

	// Second link from same parent: must reject (entry already exists), nlink unchanged.
	err = svc.Link(context.Background(), dir, "a", child)
	assert.ErrorIs(t, err, ErrEntryExists, "duplicate link in same dir must fail")
	assert.Equal(t, uint32(1), child.NLink())
}

func TestUnlinkDecrementsAndDeletes(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	child, err := NewFile("hello")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), child))

	require.NoError(t, svc.Link(context.Background(), dir, "a", child))
	assert.Equal(t, uint32(1), child.NLink())

	// Add a 2nd hardlink from another parent.
	dir2, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir2))
	require.NoError(t, svc.Link(context.Background(), dir2, "b", child))
	assert.Equal(t, uint32(2), child.NLink())

	// Unlink only decrements (nlink 2 -> 1), child remains.
	deleted, err := svc.Unlink(context.Background(), dir, "a")
	require.NoError(t, err)
	assert.Nil(t, deleted, "nlink>1 should not delete the child")
	assert.Equal(t, uint32(1), child.NLink())
	_, err = repo.Get(context.Background(), child.ID())
	assert.NoError(t, err, "child must still exist after decrement")
}

func TestUnlinkLastLinkDeletes(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	child, err := NewFile("hello")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), child))

	// Single Link -> nlink=1.
	require.NoError(t, svc.Link(context.Background(), dir, "a", child))
	assert.Equal(t, uint32(1), child.NLink())

	// Unlink the only entry: nlink 1 -> 0, child must be deleted.
	deleted, err := svc.Unlink(context.Background(), dir, "a")
	require.NoError(t, err)
	assert.NotNil(t, deleted, "nlink=1 -> 0 must return deleted child")
	assert.Equal(t, child.ID(), deleted.ID())
	_, err = repo.Get(context.Background(), child.ID())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUnlinkOrReplaceRejectsDirectory(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	parent, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), parent))

	subdir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), subdir))

	require.NoError(t, svc.Link(context.Background(), parent, "d", subdir))

	_, err = svc.UnlinkOrReplace(context.Background(), parent, "d")
	assert.ErrorIs(t, err, ErrIsDirectory, "directory target must be rejected")
}

func TestUnlinkOrReplaceNoEntry(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)

	dir, err := NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))

	deleted, err := svc.UnlinkOrReplace(context.Background(), dir, "absent")
	assert.NoError(t, err)
	assert.Nil(t, deleted)
}
