//go:build integration_ent

package ent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

func TestEnt_Node_SaveGetRoundtrip(t *testing.T) {
	ctx := context.Background()
	client := startPostgres(t)
	repo := node.NewRepository(client)

	dir, err := node.NewDirectory()
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, dir))

	got, err := repo.Get(ctx, dir.ID())
	require.NoError(t, err)
	assert.Equal(t, dir.ID(), got.ID())
	assert.Equal(t, node.NodeTypeDirectory, got.Type())
	assert.Equal(t, uint32(0), got.NLink(), "fresh inode has nlink=0")
}

func TestEnt_Node_RevisionConflict(t *testing.T) {
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

	a.SetMode(0o600)
	b.SetMode(0o700)

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
		case errors.Is(r.err, node.ErrRevisionConflict):
			conflicts++
		default:
			t.Errorf("writer %s returned unexpected error: %v", r.who, r.err)
		}
	}
	assert.Equal(t, 1, wins, "exactly one writer must win")
	assert.Equal(t, 1, conflicts, "exactly one writer must see ErrRevisionConflict")

	// The winner's mutation is the one reflected in the DB.
	final, err := repo.Get(ctx, dir.ID())
	require.NoError(t, err)
	assert.Equal(t, a.Mode()|b.Mode(), final.Mode(),
		"the winner's mode (0o600 or 0o700) must be the persisted mode")
}

func TestEnt_Node_WithTxCommit(t *testing.T) {
	ctx := context.Background()
	client := startPostgres(t)
	repo := node.NewRepository(client)

	var dirID string
	require.NoError(t, repo.WithTx(ctx, func(tx node.Repository) error {
		dir, err := node.NewDirectory()
		if err != nil {
			return err
		}
		if err := tx.Save(ctx, dir); err != nil {
			return err
		}
		dirID = dir.ID().String()
		return nil
	}))

	got, err := repo.Get(ctx, mustParseID(t, dirID))
	require.NoError(t, err)
	assert.Equal(t, node.NodeTypeDirectory, got.Type())
}

func TestEnt_Node_WithTxRollback(t *testing.T) {
	ctx := context.Background()
	client := startPostgres(t)
	repo := node.NewRepository(client)

	var dirID string
	err := repo.WithTx(ctx, func(tx node.Repository) error {
		dir, err := node.NewDirectory()
		if err != nil {
			return err
		}
		if err := tx.Save(ctx, dir); err != nil {
			return err
		}
		dirID = dir.ID().String()
		// Returning an error rolls the tx back.
		return assert.AnError
	})
	require.Error(t, err)

	// The dir must not exist after rollback.
	_, err = repo.Get(ctx, mustParseID(t, dirID))
	assert.ErrorIs(t, err, node.ErrNotFound,
		"rolled-back tx must not leave a row behind")
}
