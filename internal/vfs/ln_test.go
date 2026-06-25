package vfs

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustRootID returns the root node ID of the vfs test service.
// Tests that need to link new nodes under the root can use this
// instead of building their own root.
func mustRootID(t *testing.T, svc *Service) uuid.UUID {
	t.Helper()
	d, err := svc.DriveClient.GetByID(context.Background(), "d1")
	require.NoError(t, err)
	id := d.RootNodeID()
	require.NotNil(t, id)
	return *id
}

func TestSymlink(t *testing.T) {
	ctx := context.Background()
	svc := newTestService()

	link, err := svc.Symlink(ctx, "d1", "/target", "/link")
	require.NoError(t, err)
	assert.True(t, link.IsSymlink(), "result should be a symlink node")
}

func TestHardlink(t *testing.T) {
	ctx := context.Background()
	svc, nodeSvc := newTestServiceWithNode()
	root, err := nodeSvc.GetByID(ctx, mustRootID(t, svc))
	require.NoError(t, err)

	src, err := nodeSvc.CreateFile(ctx, "hello")
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(ctx, root, "src", src))

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
