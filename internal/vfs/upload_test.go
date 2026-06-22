package vfs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/upload"
)

type objectNotFoundStore struct {
	Store
}

func (s *objectNotFoundStore) ObjectExists(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

func TestInitiateAndCompleteUpload(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	info, err := svc.InitiateUpload(ctx, "user1", "d1", "/big.bin", strPtr("application/octet-stream"), int64Ptr(42), time.Hour)
	require.NoError(t, err)
	assert.Equal(t, "PUT", info.Method)
	assert.NotEmpty(t, info.UploadID)

	meta, err := svc.Reg.Get(ctx, info.UploadID)
	require.NoError(t, err)
	assert.Equal(t, "/big.bin", meta.Path)

	n, err := svc.CompleteUpload(ctx, "user1", "d1", info.UploadID, 42, nil)
	require.NoError(t, err)
	assert.True(t, n.IsObject())

	_, err = svc.Reg.Get(ctx, info.UploadID)
	assert.ErrorIs(t, err, upload.ErrNotFound)
}

func TestCompleteUploadSizeMismatch(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	info, err := svc.InitiateUpload(ctx, "user1", "d1", "/big.bin", nil, int64Ptr(42), time.Hour)
	require.NoError(t, err)

	_, err = svc.CompleteUpload(ctx, "user1", "d1", info.UploadID, 43, nil)
	assert.Error(t, err)
}

func TestPresignDownloadNotObject(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.Touch(ctx, "user1", "d1", "/plain.txt")
	require.NoError(t, err)

	_, err = svc.PresignDownload(ctx, "user1", "d1", "/plain.txt", time.Hour)
	assert.Error(t, err)
}

type recordingReencryptor struct {
	calls []reencryptCall
}

type reencryptCall struct {
	driveID string
	bucket  string
	srcKey  string
	dstKey  string
}

func (r *recordingReencryptor) MigratePlaintext(_ context.Context, driveID, bucket, srcKey, dstKey string) error {
	r.calls = append(r.calls, reencryptCall{driveID: driveID, bucket: bucket, srcKey: srcKey, dstKey: dstKey})
	return nil
}

func TestCompleteUpload_ReencryptsWhenReencryptorSet(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	rec := &recordingReencryptor{}
	svc.Reencryptor = rec

	info, err := svc.InitiateUpload(ctx, "user1", "d1", "/big.bin", nil, int64Ptr(42), time.Hour)
	require.NoError(t, err)

	_, err = svc.CompleteUpload(ctx, "user1", "d1", info.UploadID, 42, nil)
	require.NoError(t, err)

	require.Len(t, rec.calls, 1, "CompleteUpload should call MigratePlaintext exactly once")
	call := rec.calls[0]
	assert.Equal(t, "d1", call.driveID)
	assert.Equal(t, "b", call.bucket)
	assert.Equal(t, "drives/d1/uploads/"+info.UploadID, call.srcKey)
	assert.Equal(t, call.srcKey, call.dstKey, "in-place re-encryption")
}

func TestCompleteUpload_ObjectNotExists(t *testing.T) {
	repo := newFakeRepo()
	nodeSvc := node.NewService(repo)
	root, _ := nodeSvc.CreateDirectory(context.Background())
	d := &fakeDrive{rootID: root.ID()}
	svc := NewService(nodeSvc, d, &fakeUser{}, &objectNotFoundStore{Store: &fakeStore{}}, &fakePerm{}, nil, nil, nil, nil)

	ctx := context.Background()
	info, err := svc.InitiateUpload(ctx, "user1", "d1", "/missing.bin", nil, nil, time.Hour)
	require.NoError(t, err)

	_, err = svc.CompleteUpload(ctx, "user1", "d1", info.UploadID, 0, nil)
	assert.ErrorIs(t, err, ErrObjectNotUploaded)
}
