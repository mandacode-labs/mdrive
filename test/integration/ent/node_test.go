//go:build integration_ent

package ent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

func TestEntNodeSaveGetRoundtrip(t *testing.T) {
	ctx := context.Background()
	client := startPostgres(t)
	repo := node.NewRepository(client)

	dir, err := node.NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, dir))

	got, err := repo.Get(ctx, dir.ID())
	require.NoError(t, err)
	assert.Equal(t, dir.ID(), got.ID())
	assert.Equal(t, node.NodeKindDirectory, got.Kind())
	assert.Equal(t, uint32(0), got.NLink(), "fresh inode has nlink=0")
}

func TestEntNodeRevisionConflict(t *testing.T) {
	// Race scenario: two writers load the same inode, mutate
	// independently, and Save concurrently. The optimistic-
	// concurrency guard means exactly one Save wins; the other
	// returns ErrRevisionConflict.
	ctx := context.Background()
	client := startPostgres(t)
	repo := node.NewRepository(client)

	dir, err := node.NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, dir))

	// Two readers each load a fresh copy of the same inode.
	// Both share the same staleRev until they Save.
	a, err := repo.Get(ctx, dir.ID())
	require.NoError(t, err)
	b, err := repo.Get(ctx, dir.ID())
	require.NoError(t, err)
	require.Equal(t, a.Revision(), b.Revision(), "fresh reads should agree")

	require.NoError(t, a.WriteFile("a-wins"))
	require.NoError(t, b.WriteFile("b-wins"))

	// Run the two saves concurrently. errgroup waits for both
	// and surfaces the first non-nil error, but here we want
	// to inspect both outcomes, so use plain goroutines.
	type result struct {
		who string
		err error
	}
	done := make(chan result, 2)
	go func() { done <- result{"a", repo.Save(ctx, a)} }()
	go func() { done <- result{"b", repo.Save(ctx, b)} }()

	var wins, conflicts int
	for i := 0; i < 2; i++ {
		r := <-done
		switch {
		case r.err == nil:
			wins++
		case errorx.IsKind(r.err, errorx.KindConflict):
			conflicts++
		default:
			t.Errorf("writer %s returned unexpected error: %v", r.who, r.err)
		}
	}
	assert.Equal(t, 1, wins, "exactly one writer must win")
	assert.Equal(t, 1, conflicts, "exactly one writer must see ErrRevisionConflict")

	// The winner's content is the one reflected in the DB.
	final, err := repo.Get(ctx, dir.ID())
	require.NoError(t, err)
	got, err := final.ReadFile()
	require.NoError(t, err)
	assert.True(t, got == "a-wins" || got == "b-wins",
		"the persisted content must equal the winner's, got %q", got)
}

func TestEntNodeWithTxCommit(t *testing.T) {
	ctx := context.Background()
	client := startPostgres(t)
	repo := node.NewRepository(client)
	tm := entx.NewTxManager(client)

	var dirID string
	require.NoError(t, tm.WithTx(ctx, func(ctx context.Context) error {
		dir, err := node.NewDirectory()
		if err != nil {
			return err
		}
		if err := repo.Save(ctx, dir); err != nil {
			return err
		}
		dirID = dir.ID().String()
		return nil
	}))

	got, err := repo.Get(ctx, mustParseID(t, dirID))
	require.NoError(t, err)
	assert.Equal(t, node.NodeKindDirectory, got.Kind())
}

func TestEntNodeWithTxRollback(t *testing.T) {
	ctx := context.Background()
	client := startPostgres(t)
	repo := node.NewRepository(client)
	tm := entx.NewTxManager(client)

	var dirID string
	err := tm.WithTx(ctx, func(ctx context.Context) error {
		dir, err := node.NewDirectory()
		if err != nil {
			return err
		}
		if err := repo.Save(ctx, dir); err != nil {
			return err
		}
		dirID = dir.ID().String()
		// Returning an error rolls the tx back.
		return assert.AnError
	})
	require.Error(t, err)

	// The dir must not exist after rollback.
	_, err = repo.Get(ctx, mustParseID(t, dirID))
	assert.Equal(t, errorx.KindNotFound, errorx.KindOf(err),
		"rolled-back tx must not leave a row behind")
}
