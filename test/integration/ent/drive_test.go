//go:build integration_ent

package ent

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
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
