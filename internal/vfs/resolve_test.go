package vfs

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// driveFixture holds two drives' roots and the ids used by the
// fakeDrive. Lets us exercise cross-drive resolve without spinning
// up Postgres.
type driveFixture struct {
	repo *fakeRepo
	dA   *node.Node
	dB   *node.Node
	idA  string
	idB  string
}

func newDriveFixture(t *testing.T) (*driveFixture, *node.Service) {
	t.Helper()
	repo := newFakeRepo()
	svc := node.NewService(repo)
	a, err := svc.CreateDirectory(context.Background())
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), a))
	b, err := svc.CreateDirectory(context.Background())
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), b))
	return &driveFixture{
		repo: repo,
		dA:   a,
		dB:   b,
		idA:  "drive-A",
		idB:  "drive-B",
	}, svc
}

func (f *driveFixture) driveClient() DriveClient {
	now := time.Now()
	return &multiDriveClient{roots: map[string]uuid.UUID{
		f.idA: f.dA.ID(),
		f.idB: f.dB.ID(),
	}, now: now}
}

type multiDriveClient struct {
	roots map[string]uuid.UUID
	now   time.Time
}

func (m *multiDriveClient) GetByID(_ context.Context, id string) (*drive.Drive, error) {
	rootID, ok := m.roots[id]
	if !ok {
		return nil, nil
	}
	root := rootID
	return drive.NewDrive(id, id, "d", nil, drive.ProviderS3, "owner", &root, nil, m.now, m.now), nil
}
func (m *multiDriveClient) GetByPublicID(_ context.Context, _ string) (*drive.Drive, error) {
	return nil, nil
}
func (m *multiDriveClient) GetStorage(_ context.Context, _, _ string) (*drive.Storage, error) {
	return nil, nil
}

// TestResolveCrossDrive: a path through a mount resolves to the
// source drive's node.
func TestResolveCrossDrive(t *testing.T) {
	fix, nodeSvc := newDriveFixture(t)
	// A:/mounts/team -> B
	mounts, err := nodeSvc.CreateDirectory(context.Background())
	require.NoError(t, err)
	require.NoError(t, fix.repo.Save(context.Background(), mounts))
	require.NoError(t, nodeSvc.Link(context.Background(), fix.dA, "mounts", mounts))

	mount, err := nodeSvc.CreateMount(context.Background(), fix.idB)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(context.Background(), mounts, "team", mount))

	// B:/shared.txt
	fileB, err := nodeSvc.CreateFile(context.Background(), "x")
	require.NoError(t, err)
	require.NoError(t, fix.repo.Save(context.Background(), fileB))
	require.NoError(t, nodeSvc.Link(context.Background(), fix.dB, "shared.txt", fileB))

	svc := NewService(ServiceConfig{
		Node:  nodeSvc,
		Drive: fix.driveClient(),
		Store: &fakeStore{},
	})

	res, err := svc.Resolve(context.Background(), fix.idA, "/mounts/team/shared.txt")
	require.NoError(t, err)
	assert.Equal(t, fix.idB, res.DriveID)
	assert.Equal(t, fileB.ID(), res.Node.ID())
}

// TestResolveMountCycle: A -> B -> A is rejected as ErrMountCycle.
func TestResolveMountCycle(t *testing.T) {
	fix, nodeSvc := newDriveFixture(t)
	// A:/mount -> B
	mount, err := nodeSvc.CreateMount(context.Background(), fix.idB)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(context.Background(), fix.dA, "mount", mount))

	// B:/back -> A
	back, err := nodeSvc.CreateMount(context.Background(), fix.idA)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(context.Background(), fix.dB, "back", back))

	svc := NewService(ServiceConfig{
		Node:  nodeSvc,
		Drive: fix.driveClient(),
		Store: &fakeStore{},
	})

	_, err = svc.Resolve(context.Background(), fix.idA, "/mount/back/mount")
	assert.ErrorIs(t, err, ErrMountCycle)
}
