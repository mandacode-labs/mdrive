package vfs_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

func TestSymlink(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)

	link, err := svc.Symlink(ctx, "d1", "/target", "/link")
	require.NoError(t, err)
	assert.True(t, link.IsSymlink(), "result should be a symlink node")
}

func TestHardlink(t *testing.T) {
	ctx := context.Background()
	root, driveState, repo := setupRoot(t)
	nodeMock := newMockNodeClient(t, repo)
	driveMock := newMockDriveClient(t, driveState)
	garbageMock := newMockGarbageRecorder(t)
	tmMock := newMockTxManager(t)
	fullSvc := vfs.NewService(vfs.Config{
		NodeClient:      nodeMock,
		DriveClient:     driveMock,
		GarbageRecorder: garbageMock,
		TxManager:       tmMock,
	})

	src, err := node.NewFile("hello")
	require.NoError(t, err)
	require.NoError(t, nodeMock.Link(ctx, root, "src", src))

	link, err := fullSvc.Hardlink(ctx, "d1", "/src", "/hard")
	require.NoError(t, err)
	assert.Equal(t, src.ID(), link.ID(), "hardlink should share the source inode")
	assert.Equal(t, uint32(2), link.NLink(), "nlink should be incremented")
}

func TestHardlinkRejectsDirectory(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(t)
	_, err := svc.Hardlink(ctx, "d1", "/", "/hard")
	assert.ErrorIs(t, err, vfs.ErrHardlinkNotSupported)
}

// silence unused-import warning when mock is not referenced
var _ = mock.Anything
