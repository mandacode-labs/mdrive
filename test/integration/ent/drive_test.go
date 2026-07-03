//go:build integration_ent

package ent

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	driveMocks "github.com/mandacode-labs/mdrive/internal/core/drive/mocks"
	corenode "github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/crypto"
	"github.com/mandacode-labs/mdrive/internal/entx"
)

func TestEntDriveSoftDeleteAndRestore(t *testing.T) {
	ctx := context.Background()
	client := startPostgres(t)
	repo := drive.NewRepository(client, nil)

	id := ulidLike()
	pubID := "pub-" + id
	d := drive.NewDrive(id, pubID, "test-drive", nil, drive.ProviderS3, "owner-1", nil, nil, time.Now(), time.Now())
	storage := drive.NewStorage(id, "bucket", nil, "us-east-1", "ak", "sk", false)

	require.NoError(t, repo.Create(ctx, d, storage))

	// Drive is visible before delete.
	got, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, got.DeletedAt(), "freshly created drive is not soft-deleted")

	// Soft-delete.
	require.NoError(t, repo.SoftDelete(ctx, id))

	got, err = repo.GetByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, got.DeletedAt(), "soft-deleted drive has DeletedAt set")

	// Restore.
	require.NoError(t, repo.Restore(ctx, id))

	got, err = repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, got.DeletedAt(), "restored drive is no longer soft-deleted")
}

func TestEntDriveListDeletedReturnsOnlyDeleted(t *testing.T) {
	ctx := context.Background()
	client := startPostgres(t)
	repo := drive.NewRepository(client, nil)

	ownerID := "owner-list"
	id1 := ulidLike()
	id2 := ulidLike()
	for _, id := range []string{id1, id2} {
		d := drive.NewDrive(id, "pub-"+id, "d", nil, drive.ProviderS3, ownerID, nil, nil, time.Now(), time.Now())
		storage := drive.NewStorage(id, "bucket", nil, "us-east-1", "ak", "sk", false)
		require.NoError(t, repo.Create(ctx, d, storage))
	}
	require.NoError(t, repo.SoftDelete(ctx, id2))

	deleted, err := repo.FindDeleted(ctx, time.Now().Add(time.Hour), 100)
	require.NoError(t, err)
	found := false
	for _, d := range deleted {
		if d.ID() == id2 {
			found = true
		}
		assert.NotEqual(t, id1, d.ID(),
			"non-deleted drive must not appear in ListDeleted")
	}
	assert.True(t, found, "soft-deleted drive must appear in ListDeleted")
}

// ulidLike returns a 26-char random ID to mimic the production
// ULID generator. Ent stores the value as a string column; the
// only requirement is that it be unique within a test run.
func ulidLike() string {
	u := uuid.New()
	const hex = "0123456789abcdef"
	out := make([]byte, 26)
	copy(out, u.String())
	// Pad to 26 chars if the uuid string is shorter (uuid.String
	// is 36 chars, so trim and pad as needed).
	for i := 0; i < 26; i++ {
		out[i] = hex[int(out[i])%16]
	}
	return string(out)
}

// TestEntDriveCreatePersistsAndListsByOwner reproduces the production
// handler path: a drive is created through the repository (single
// non-tx call, like drive.repo.Create inside the WithTx closure),
// then a separate ListByOwner query must return it. This pins down
// the contract that an INSERT immediately visible to a follow-up
// SELECT — the failure mode seen in production where the POST
// returned a populated drive object but GET /v1/drives returned []
// indicates the row never reached the table the GET queries.
func TestEntDriveCreatePersistsAndListsByOwner(t *testing.T) {
	ctx := context.Background()
	client := startPostgres(t)
	repo := drive.NewRepository(client, crypto.NoOp{})

	ownerID := ulidLike()
	id := ulidLike()
	now := time.Now()
	d := drive.NewDrive(id, "pub-"+id, "persist-drive", nil, drive.ProviderS3, ownerID, nil, nil, now, now)
	storage := drive.NewStorage(id, "bucket", nil, "us-east-1", "ak", "sk", false)

	require.NoError(t, repo.Create(ctx, d, storage),
		"Create must succeed against a freshly-migrated Postgres")

	got, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, got, "drive row must be visible to GetByID immediately after Create")
	assert.Equal(t, id, got.ID())
	assert.Equal(t, ownerID, got.OwnerID())

	listed, err := repo.FindByOwner(ctx, ownerID)
	require.NoError(t, err)
	found := false
	for _, d := range listed {
		if d.ID() == id {
			found = true
			break
		}
	}
	assert.True(t, found,
		"FindByOwner must return the drive inserted via Create — "+
			"production saw POST 200 with a populated body but GET /v1/drives []; "+
			"this test guards against that gap by exercising the same insert+select path")
}

// TestEntDriveCreateInsideWithTxPersists reproduces the production
// service.Create path: drive INSERT and storage INSERT run inside a
// single WithTx closure. After Commit, the drive must be visible to
// a follow-up FindByOwner from the same repository. This is the
// exact code path CreateDrive handler hits via drive.Service.Create.
func TestEntDriveCreateInsideWithTxPersists(t *testing.T) {
	ctx := context.Background()
	client := startPostgres(t)
	repo := drive.NewRepository(client, crypto.NoOp{})

	ownerID := ulidLike()
	id := ulidLike()
	now := time.Now()
	d := drive.NewDrive(id, "pub-"+id, "tx-drive", nil, drive.ProviderS3, ownerID, nil, nil, now, now)
	storage := drive.NewStorage(id, "bucket", nil, "us-east-1", "ak", "sk", false)

	require.NoError(t, entx.NewTxManager(client).WithTx(ctx, func(ctx context.Context) error {
		return repo.Create(ctx, d, storage)
	}))

	listed, err := repo.FindByOwner(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, listed, 1,
		"drive inserted inside a committed WithTx must be visible to FindByOwner")
	assert.Equal(t, id, listed[0].ID())

	storageGot, err := repo.GetStorage(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, storageGot,
		"storage row inserted inside the same tx must also be visible")
	assert.Equal(t, "bucket", storageGot.Bucket())
}

// TestEntDriveServiceCreateEndToEnd mirrors what the HTTP handler
// does: build a *drive.Service with a real ent-backed repo and a
// rootNodeCreator wired to a real node.Service, call Create, then
// ListByOwner. This catches the regression where the handler
// returned a populated drive object but the row was missing from
// the GET /v1/drives query.
func TestEntDriveServiceCreateEndToEnd(t *testing.T) {
	ctx := context.Background()
	client := startPostgres(t)
	driveRepo := drive.NewRepository(client, crypto.NoOp{})
	nodeRepo := corenode.NewRepository(client)
	nodeSvc := corenode.NewService(nodeRepo, entx.NewTxManager(client))

	owner := driveMocks.NewOwnerCheckerMock(t)
	owner.EXPECT().Exist(mock.Anything, mock.Anything).Return(true, nil).Maybe()
	svc := drive.NewService(driveRepo, owner, rootNodeAdapter{svc: nodeSvc}, entx.NewTxManager(client))

	ownerID := ulidLike() // 26-char ULID; owner_id column is character(32)
	created, _, err := svc.Create(ctx, ownerID, "e2e-drive", "desc", drive.StorageConfig{
		Bucket:    "bucket",
		Region:    "us-east-1",
		AccessKey: "ak",
		SecretKey: "sk",
	})
	require.NoError(t, err)
	require.NotNil(t, created,
		"Service.Create must return a non-nil drive on success")

	listed, err := svc.ListByOwner(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, listed, 1,
		"a drive created via Service.Create must show up in ListByOwner — "+
			"production symptom: POST 200 with body but GET /v1/drives returned []")
	assert.Equal(t, created.ID(), listed[0].ID())
	assert.Equal(t, "e2e-drive", listed[0].Name())
}

// alwaysExistOwner removed in favor of driveMocks.NewOwnerCheckerMock.
// rootNodeAdapter wires node.Service to the drive.RootDirectoryCreator
// shape production uses in app.go.
type rootNodeAdapter struct {
	svc *corenode.Service
}

func (a rootNodeAdapter) CreateRootDirectory(ctx context.Context) (uuid.UUID, error) {
	root, err := a.svc.CreateDirectory(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return root.ID(), nil
}
