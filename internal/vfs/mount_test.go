package vfs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
)

func TestMount_SelfMount_Rejected(t *testing.T) {
	ctx := context.Background()
	svc := newTestService()
	err := svc.Mount(ctx, "d1", "/sub", "d1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot mount a drive onto itself")
}

func TestMount_SourceNotFound_Rejected(t *testing.T) {
	ctx := context.Background()
	svc := newTestService()
	// fakeDrive.GetByID ignores the id, so swap it for one that
	// returns a "not found" error.
	svc.Drive = &errDrive{err: drive.ErrNotFound}
	err := svc.Mount(ctx, "d1", "/sub", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source drive lookup")
}

func TestMount_SourceSoftDeleted_Rejected(t *testing.T) {
	ctx := context.Background()
	svc := newTestService()
	deletedAt := time.Now()
	rootID, err := svc.rootNodeID(ctx, "d1")
	require.NoError(t, err)
	svc.Drive = &fakeDrive{rootID: rootID, deletedAt: &deletedAt}
	err = svc.Mount(ctx, "d1", "/sub", "d2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "soft-deleted")
}

// mustDirEntries returns the entries of the directory at path.
// Used by mount tests to verify the linked node.
func mustDirEntries(t *testing.T, svc *Service, ctx context.Context, driveID, path string) []node.DirEntry {
	t.Helper()
	dir, err := svc.Ls(ctx, driveID, path)
	require.NoError(t, err)
	return dir.Entries
}

func TestMount_Ok(t *testing.T) {
	ctx := context.Background()
	svc := newTestService()
	require.NoError(t, svc.Mount(ctx, "d1", "/sub", "d2"))

	// The mount is a directory entry at the root. List the root
	// and confirm the mount is there.
	entries := mustDirEntries(t, svc, ctx, "d1", "/")
	require.Len(t, entries, 1)
	nodeSvc := svc.Node
	mount, err := nodeSvc.GetByID(ctx, entries[0].InodeID)
	require.NoError(t, err)
	assert.True(t, mount.IsMount())
}

type errDrive struct{ err error }

func (d *errDrive) GetByID(context.Context, string) (*drive.Drive, error) { return nil, d.err }
func (d *errDrive) GetByPublicID(context.Context, string) (*drive.Drive, error) {
	return nil, d.err
}
func (d *errDrive) GetStorage(context.Context, string) (*drive.Storage, error) {
	return nil, d.err
}

// unused — keeps the import set stable for any future test that
// wants to assert on a wrapped error.
