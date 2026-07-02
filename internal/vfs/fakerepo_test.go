package vfs

import (
	"context"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// TestFakeRepo_ConflictOnStaleRev proves that the test fake enforces
// optimistic concurrency: if a caller saves a node whose staleRev no
// longer matches the stored rev, the fake returns ErrRevisionConflict.
// Without this enforcement, bugs like the nlink==1 Unlink+Link hazard
// (the original Mv bug) would slip through unit tests.
func TestFakeRepoConflictOnStaleRev(t *testing.T) {
	repo := newFakeRepo()
	svc := node.NewService(repo)
	ctx := context.Background()

	root, err := svc.CreateDirectory(ctx)
	require.NoError(t, err)

	// Snapshot the root as it is now (staleRev == stored rev).
	loaded, err := svc.GetByID(ctx, root.ID())
	require.NoError(t, err)

	// Someone else loads, mutates, and saves the root.
	loaded2, err := svc.GetByID(ctx, root.ID())
	require.NoError(t, err)
	require.NoError(t, loaded2.WriteDir(node.DirContent{})) // bumps rev
	require.NoError(t, repo.Save(ctx, loaded2))

	// The first snapshot, unchanged, is now stale.
	err = repo.Save(ctx, loaded)
	assertKind(t, err, errorx.KindConflict)
}

// TestFakeRepo_GetReturnsIsolatedCopy proves that Get hands back a
// deep copy: mutating the returned node does not leak into the stored
// version. Two callers can Get the same node, mutate independently,
// and Save without surprise overrides.
func TestFakeRepoGetReturnsIsolatedCopy(t *testing.T) {
	repo := newFakeRepo()
	svc := node.NewService(repo)
	ctx := context.Background()

	n, err := svc.CreateFile(ctx, "hello")
	require.NoError(t, err)

	from1, err := svc.GetByID(ctx, n.ID())
	require.NoError(t, err)
	from2, err := svc.GetByID(ctx, n.ID())
	require.NoError(t, err)

	require.NoError(t, from1.WriteFile("world"))

	raw, err := from2.ReadFile()
	require.NoError(t, err)
	assert.Equal(t, "hello", string(raw), "second Get must not see the first caller's mutation")
}
