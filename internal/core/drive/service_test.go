package drive_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	driveMocks "github.com/mandacode-labs/mdrive/internal/core/drive/mocks"
)

func newTxManagerMock(t *testing.T) *driveMocks.TxManagerMock {
	t.Helper()
	m := driveMocks.NewTxManagerMock(t)
	m.EXPECT().WithTx(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		},
	).Maybe()
	return m
}

func newOwnerChecker(t *testing.T, exists bool) *driveMocks.OwnerCheckerMock {
	t.Helper()
	m := driveMocks.NewOwnerCheckerMock(t)
	m.EXPECT().Exist(mock.Anything, mock.Anything).Return(exists, nil).Maybe()
	return m
}

func newRootDirCreator(t *testing.T) (*driveMocks.RootDirectoryCreatorMock, uuid.UUID) {
	t.Helper()
	rootID := uuid.New()
	m := driveMocks.NewRootDirectoryCreatorMock(t)
	m.EXPECT().CreateRootDirectory(mock.Anything).Return(rootID, nil).Maybe()
	return m, rootID
}

func newRepo(t *testing.T) *driveMocks.RepositoryMock {
	return driveMocks.NewRepositoryMock(t)
}

// TestCreate_HappyPath verifies the full drive creation flow:
// owner check → root node creation → tx (Create + Update).
func TestCreate_HappyPath(t *testing.T) {
	repo := newRepo(t)
	owner := newOwnerChecker(t, true)
	rootCreator, rootID := newRootDirCreator(t)
	tm := newTxManagerMock(t)

	repo.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	repo.EXPECT().Update(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, d *drive.Drive) (*drive.Drive, error) {
			return d, nil
		},
	).Once()

	svc := drive.NewService(repo, owner, rootCreator, tm)
	dr, id, err := svc.Create(context.Background(), "user-1", "my-drive", "desc", drive.StorageConfig{
		Bucket: "b", Region: "r", AccessKey: "ak", SecretKey: "sk",
	})
	require.NoError(t, err)
	assert.Equal(t, rootID, id)
	assert.Equal(t, "my-drive", dr.Name())
	require.NotNil(t, dr.RootNodeID())
	assert.Equal(t, rootID, *dr.RootNodeID())
}

// TestCreate_RejectsUnknownOwner verifies the owner existence gate.
func TestCreate_RejectsUnknownOwner(t *testing.T) {
	repo := newRepo(t)
	owner := newOwnerChecker(t, false)
	rootCreator, _ := newRootDirCreator(t)
	tm := newTxManagerMock(t)

	svc := drive.NewService(repo, owner, rootCreator, tm)
	_, _, err := svc.Create(context.Background(), "missing", "n", "d", drive.StorageConfig{
		Bucket: "b", Region: "r", AccessKey: "ak", SecretKey: "sk",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "owner not found")
}

// TestCreate_RejectsInvalidName verifies the empty-name guard.
func TestCreate_RejectsInvalidName(t *testing.T) {
	repo := newRepo(t)
	owner := newOwnerChecker(t, true)
	rootCreator, _ := newRootDirCreator(t)
	tm := newTxManagerMock(t)

	svc := drive.NewService(repo, owner, rootCreator, tm)
	_, _, err := svc.Create(context.Background(), "user-1", "", "d", drive.StorageConfig{
		Bucket: "b", Region: "r", AccessKey: "ak", SecretKey: "sk",
	})
	require.Error(t, err)
}

// TestCreate_RejectsEmptyStorage verifies the storage config guard.
func TestCreate_RejectsEmptyStorage(t *testing.T) {
	repo := newRepo(t)
	owner := newOwnerChecker(t, true)
	rootCreator, _ := newRootDirCreator(t)
	tm := newTxManagerMock(t)

	svc := drive.NewService(repo, owner, rootCreator, tm)
	_, _, err := svc.Create(context.Background(), "user-1", "n", "d", drive.StorageConfig{})
	require.Error(t, err)
}

// TestCreate_TxErrorPropagates verifies that a repository error inside
// the transaction is surfaced to the caller.
func TestCreate_TxErrorPropagates(t *testing.T) {
	repo := newRepo(t)
	owner := newOwnerChecker(t, true)
	rootCreator, _ := newRootDirCreator(t)
	tm := newTxManagerMock(t)

	want := errors.New("db down")
	repo.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(want).Once()

	svc := drive.NewService(repo, owner, rootCreator, tm)
	_, _, err := svc.Create(context.Background(), "user-1", "n", "d", drive.StorageConfig{
		Bucket: "b", Region: "r", AccessKey: "ak", SecretKey: "sk",
	})
	require.Error(t, err)
}

// TestGetByID_NotFound verifies the nil → NotFound translation.
func TestGetByID_NotFound(t *testing.T) {
	repo := newRepo(t)
	owner := newOwnerChecker(t, true)
	rootCreator, _ := newRootDirCreator(t)
	tm := newTxManagerMock(t)

	repo.EXPECT().GetByID(mock.Anything, "x").Return(nil, nil).Once()

	svc := drive.NewService(repo, owner, rootCreator, tm)
	_, err := svc.GetByID(context.Background(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestUpdate_LeavesNameUnchangedOnEmpty verifies the empty-means-keep semantic.
func TestUpdate_LeavesNameUnchangedOnEmpty(t *testing.T) {
	repo := newRepo(t)
	owner := newOwnerChecker(t, true)
	rootCreator, _ := newRootDirCreator(t)
	tm := newTxManagerMock(t)

	existing := drive.NewDrive("d1", "p1", "orig", nil, drive.ProviderS3, "u", nil, nil, time.Now(), time.Now())
	repo.EXPECT().GetByID(mock.Anything, "d1").Return(existing, nil).Once()
	repo.EXPECT().Update(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, d *drive.Drive) (*drive.Drive, error) { return d, nil },
	).Once()

	svc := drive.NewService(repo, owner, rootCreator, tm)
	updated, err := svc.Update(context.Background(), "d1", "", "newdesc")
	require.NoError(t, err)
	assert.Equal(t, "orig", updated.Name())
	require.NotNil(t, updated.Description())
	assert.Equal(t, "newdesc", *updated.Description())
}

// TestDelete_RejectsUnknown verifies the existence pre-check.
func TestDelete_RejectsUnknown(t *testing.T) {
	repo := newRepo(t)
	owner := newOwnerChecker(t, true)
	rootCreator, _ := newRootDirCreator(t)
	tm := newTxManagerMock(t)

	repo.EXPECT().GetByID(mock.Anything, "missing").Return(nil, nil).Once()

	svc := drive.NewService(repo, owner, rootCreator, tm)
	err := svc.Delete(context.Background(), "missing")
	require.Error(t, err)
}

// TestRestore_RejectsActive verifies the not-deleted guard.
func TestRestore_RejectsActive(t *testing.T) {
	repo := newRepo(t)
	owner := newOwnerChecker(t, true)
	rootCreator, _ := newRootDirCreator(t)
	tm := newTxManagerMock(t)

	active := drive.NewDrive("d1", "p1", "n", nil, drive.ProviderS3, "u", nil, nil, time.Now(), time.Now())
	repo.EXPECT().GetByID(mock.Anything, "d1").Return(active, nil).Once()

	svc := drive.NewService(repo, owner, rootCreator, tm)
	_, err := svc.Restore(context.Background(), "d1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not deleted")
}

// TestListDeletedForAdmin_RejectsNonAdmin verifies the admin gate.
func TestListDeletedForAdmin_RejectsNonAdmin(t *testing.T) {
	repo := newRepo(t)
	owner := newOwnerChecker(t, true)
	rootCreator, _ := newRootDirCreator(t)
	tm := newTxManagerMock(t)

	svc := drive.NewService(repo, owner, rootCreator, tm)
	_, err := svc.ListDeletedForAdmin(context.Background(), false, time.Now(), 10)
	require.Error(t, err)
}
