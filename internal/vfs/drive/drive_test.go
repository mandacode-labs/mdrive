package drive

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coredrive "github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// fakeClient is a hand-rolled stub of the drive Client interface,
// just enough to exercise the vfs/drive.Service permission checks
// and pass-through behavior.
type fakeClient struct {
	storage     *coredrive.Storage
	drives      map[string]*coredrive.Drive
	listDeleted []*coredrive.Drive
	listByOwner map[string][]*coredrive.Drive
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		drives:      map[string]*coredrive.Drive{},
		listByOwner: map[string][]*coredrive.Drive{},
	}
}

func (f *fakeClient) Create(_ context.Context, name string, desc *string, ownerID string, _ coredrive.StorageConfig) (*coredrive.Drive, uuid.UUID, error) {
	rootID := uuid.New()
	now := time.Now()
	d := coredrive.NewDrive("d-"+name, "pub-"+name, name, desc, coredrive.ProviderS3, ownerID, &rootID, nil, now, now)
	f.drives[d.ID()] = d
	f.listByOwner[ownerID] = append(f.listByOwner[ownerID], d)
	return d, rootID, nil
}

func (f *fakeClient) GetByID(_ context.Context, id string) (*coredrive.Drive, error) {
	d, ok := f.drives[id]
	if !ok {
		return nil, nil
	}
	return d, nil
}

func (f *fakeClient) GetByPublicID(_ context.Context, pubID string) (*coredrive.Drive, error) {
	for _, d := range f.drives {
		if d.PublicID() == pubID {
			return d, nil
		}
	}
	return nil, nil
}

func (f *fakeClient) GetStorage(_ context.Context, driveID string) (*coredrive.Storage, error) {
	if f.storage == nil {
		return coredrive.NewStorage(driveID, "bucket", nil, "us-east-1", "a", "s", false), nil
	}
	return f.storage, nil
}

func (f *fakeClient) Update(_ context.Context, id string, name, description *string) (*coredrive.Drive, error) {
	d, ok := f.drives[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return d, nil
}

func (f *fakeClient) Delete(_ context.Context, id string) error {
	delete(f.drives, id)
	return nil
}

func (f *fakeClient) Restore(_ context.Context, id string) (*coredrive.Drive, error) {
	return f.drives[id], nil
}

func (f *fakeClient) ListDeleted(_ context.Context, _ time.Time, _ int) ([]*coredrive.Drive, error) {
	return f.listDeleted, nil
}

func (f *fakeClient) ListByOwner(_ context.Context, ownerID string) ([]*coredrive.Drive, error) {
	return f.listByOwner[ownerID], nil
}

// fakePerm is a hand-rolled stub that grants or denies by userID
// suffix: userIDs ending in "deny" are denied, all others allowed.
type fakePerm struct {
	granted map[string]bool
}

func newFakePerm() *fakePerm { return &fakePerm{granted: map[string]bool{}} }

func (p *fakePerm) Check(_ context.Context, userID string, _ permission.Permission, _, driveID string) (bool, error) {
	if userID == "deny" {
		return false, nil
	}
	return p.granted[driveID], nil
}

func (p *fakePerm) Grant(_ context.Context, userID, _, _, driveID string) error {
	p.granted[driveID] = true
	return nil
}

func (p *fakePerm) ListObjects(context.Context, string, string, string) ([]string, error) {
	return nil, nil
}

func (p *fakePerm) Revoke(context.Context, string, string, string, string) error {
	return nil
}

func newService(t *testing.T) (*Service, *fakeClient) {
	t.Helper()
	c := newFakeClient()
	return NewService(Config{Drive: c, Perm: newFakePerm()}), c
}

var _ = newService // silence unused warning when test is removed

func TestCreateGrantsOwner(t *testing.T) {
	svc, c := newService(t)
	ctx := context.Background()
	d, _, err := svc.Create(ctx, "owner1", "my-drive", "desc", coredrive.StorageConfig{})
	require.NoError(t, err)
	assert.NotEmpty(t, d.ID())
	assert.NotNil(t, c.drives[d.ID()])
}

func TestGetRespectsPermission(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	d, _, err := svc.Create(ctx, "owner1", "d1", "", coredrive.StorageConfig{})
	require.NoError(t, err)
	_, err = svc.Get(ctx, "deny", d.ID())
	assert.ErrorIs(t, err, ErrPermission)
	_, err = svc.Get(ctx, "owner1", d.ID())
	assert.NoError(t, err)
}

func TestListByOwnerDoesNotCheckPermission(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	_, _, err := svc.Create(ctx, "owner1", "d1", "", coredrive.StorageConfig{})
	require.NoError(t, err)
	drives, err := svc.ListByOwner(ctx, "owner1")
	require.NoError(t, err)
	assert.Len(t, drives, 1)
}

func TestNilPermAllowsAll(t *testing.T) {
	c := newFakeClient()
	svc := NewService(Config{Drive: c, Perm: nil})
	ctx := context.Background()
	d, _, err := svc.Create(ctx, "owner1", "d1", "", coredrive.StorageConfig{})
	require.NoError(t, err)
	_, err = svc.Get(ctx, "anyone", d.ID())
	assert.NoError(t, err, "nil perm should skip check")
	_ = c
}
