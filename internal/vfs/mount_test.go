package vfs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// TestMountRequiresViewOnSource verifies that creating a mount is
// rejected when the caller lacks view permission on the source drive.
func TestMountRequiresViewOnSource(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(ServiceConfig{
		Node:  node.NewService(repo),
		Drive: &fakeDrive{},
		User:  &fakeUser{},
		Store: &fakeStore{},
		Perm:  &denyPerm{},
	})
	_, err := svc.Mount(context.Background(), "user1", "d1", "/mounts/team", "d2")
	assert.Error(t, err)
}

type denyPerm struct{}

func (d *denyPerm) Check(_ context.Context, _ string, _ permission.Permission, _, _ string) (bool, error) {
	return false, nil
}
func (d *denyPerm) Grant(_ context.Context, _, _, _, _ string) error         { return nil }

// TestUnmountNotAMount rejects unmount on a non-mount entry.
func TestUnmountNotAMount(t *testing.T) {
	repo := newFakeRepo()
	nodeSvc := node.NewService(repo)
	root, err := nodeSvc.CreateDirectory(context.Background())
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), root))

	drive := &fakeDrive{rootID: root.ID()}
	svc := NewService(ServiceConfig{
		Node:  nodeSvc,
		Drive: drive,
		User:  &fakeUser{},
		Store: &fakeStore{},
		Perm:  &fakePerm{},
	})

	// Create a regular file under root.
	f, err := nodeSvc.CreateFile(context.Background(), "x")
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), f))
	require.NoError(t, nodeSvc.Link(context.Background(), root, "a", f))

	err = svc.Unmount(context.Background(), "user1", "d1", "/a")
	assert.Error(t, err)
}
