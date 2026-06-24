package vfs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

func TestSymlink(t *testing.T) {
	ctx := context.Background()
	svc := newTestService()

	link, err := svc.Symlink(ctx, "d1", "/target", "/link")
	require.NoError(t, err)
	assert.True(t, link.IsSymlink(), "result should be a symlink node")
}

func TestHardlink(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	nodeSvc := node.NewService(repo)
	root, err := nodeSvc.CreateDirectory(ctx)
	require.NoError(t, err)
	svc := NewService(ServiceConfig{
		Node:  nodeSvc,
		Drive: &fakeDrive{rootID: root.ID()},
		Store: &fakeStore{},
	})

	src, err := svc.Node.CreateFile(ctx, "hello")
	require.NoError(t, err)
	require.NoError(t, repo.Save(ctx, src))
	require.NoError(t, svc.Node.Link(ctx, root, "src", src))

	link, err := svc.Hardlink(ctx, "d1", "/src", "/hard")
	require.NoError(t, err)
	assert.Equal(t, src.ID(), link.ID(), "hardlink should share the source inode")
	assert.Equal(t, uint32(2), link.NLink(), "nlink should be incremented")
}

func TestHardlinkRejectsDirectory(t *testing.T) {
	ctx := context.Background()
	svc := newTestService()
	_, err := svc.Hardlink(ctx, "d1", "/", "/hard")
	assert.ErrorIs(t, err, ErrHardlinkNotSupported)
}
