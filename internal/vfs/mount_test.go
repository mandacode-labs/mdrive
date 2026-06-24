package vfs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
)

func TestMount_SelfMount_Rejected(t *testing.T) {
	ctx := context.Background()
	svc := newTestService()
	_, err := svc.Mount(ctx, "d1", "/sub", "d1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot mount a drive onto itself")
}

func TestMount_SourceNotFound_Rejected(t *testing.T) {
	ctx := context.Background()
	svc := newTestService()
	// fakeDrive.GetByID ignores the id, so swap it for one that
	// returns a "not found" error.
	svc.Drive = &errDrive{err: drive.ErrNotFound}
	_, err := svc.Mount(ctx, "d1", "/sub", "missing")
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
	_, err = svc.Mount(ctx, "d1", "/sub", "d2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "soft-deleted")
}

func TestMount_Ok(t *testing.T) {
	ctx := context.Background()
	svc := newTestService()
	m, err := svc.Mount(ctx, "d1", "/sub", "d2")
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.True(t, m.IsMount())
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
var _ = errors.New
