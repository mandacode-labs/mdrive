package upload_test

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
	"github.com/mandacode-labs/mdrive/internal/core/node"
	nodeMocks "github.com/mandacode-labs/mdrive/internal/core/node/mocks"
	entxMocks "github.com/mandacode-labs/mdrive/internal/entx/mocks"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/upload"
	uploadMocks "github.com/mandacode-labs/mdrive/internal/upload/mocks"
	vfsMocks "github.com/mandacode-labs/mdrive/internal/vfs/mocks"
)

func newTestService(t *testing.T, reg upload.TokenRegistry) upload.Service {
	t.Helper()
	if reg == nil {
		reg = upload.NewMemoryRegistry()
	}
	store := uploadMocks.NewObjectStoreMock(t)
	store.EXPECT().GetPresignedUploadURL(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("https://s3.example.com/upload", nil).Maybe()
	store.EXPECT().GetPresignedDownloadURL(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("https://s3.example.com/download", nil).Maybe()
	store.EXPECT().ObjectExists(mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil).Maybe()
	store.EXPECT().DeleteObject(mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()

	storage := drive.NewStorage("d1", "my-bucket", nil, "us-east-1", "a", "s", false)
	drives := newDriveMock(t, storage)
	nodes := newNodeMock(t)
	path := newVfsMock(t)

	tm := newTxMock(t)

	return upload.NewService(upload.Config{
		TokenRegistry: reg,
		StorageLookup: drives,
		NodeLifecycle: nodes,
		ObjectStore:   store,
		Path:          path,
		TxManager:     tm,
	})
}

func newDriveMock(t *testing.T, storage *drive.Storage) *driveMocks.ServiceMock {
	t.Helper()
	m := driveMocks.NewServiceMock(t)
	m.EXPECT().GetStorage(mock.Anything, mock.Anything).Return(storage, nil).Maybe()
	return m
}

func newNodeMock(t *testing.T) *nodeMocks.NodeOperationMock {
	t.Helper()
	root, _ := node.NewDirectory()
	obj, _ := node.NewObject(node.ObjectContent{Bucket: "b", Key: "k"}, 100)
	m := nodeMocks.NewNodeOperationMock(t)
	m.EXPECT().CreateObject(mock.Anything, mock.Anything, mock.Anything).Return(obj, nil).Maybe()
	m.EXPECT().Link(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	m.EXPECT().Delete(mock.Anything, mock.Anything).Return(nil).Maybe()
	m.EXPECT().GetByID(mock.Anything, mock.Anything).Return(root, nil).Maybe()
	return m
}

func newVfsMock(t *testing.T) *vfsMocks.ServiceMock {
	t.Helper()
	root, _ := node.NewDirectory()
	m := vfsMocks.NewServiceMock(t)
	m.EXPECT().GetRootNodeID(mock.Anything, mock.Anything).Return(root.ID(), nil).Maybe()
	m.EXPECT().ResolveParentNodeID(mock.Anything, mock.Anything, mock.Anything).Return(uuid.Nil, "test.bin", nil).Maybe()
	m.EXPECT().ResolveNodeID(mock.Anything, mock.Anything, mock.Anything).Return(uuid.Nil, nil).Maybe()
	return m
}

func newTxMock(t *testing.T) *entxMocks.TxManagerMock {
	t.Helper()
	m := entxMocks.NewTxManagerMock(t)
	m.EXPECT().WithTx(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		},
	).Maybe()
	return m
}

func TestInitiateUploadHappyPath(t *testing.T) {
	svc := newTestService(t, nil)
	info, err := svc.InitiateUpload(context.Background(), "u1", "d1", "/test.bin", nil, nil, time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, info.UploadID)
	assert.Equal(t, "PUT", info.Method)
	assert.Equal(t, "https://s3.example.com/upload", info.URL)
}

func TestInitiateUploadPresignFailureRollsBackToken(t *testing.T) {
	reg := upload.NewMemoryRegistry()
	store := uploadMocks.NewObjectStoreMock(t)
	store.EXPECT().GetPresignedUploadURL(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", errors.New("presign fail")).Maybe()
	store.EXPECT().GetPresignedDownloadURL(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", nil).Maybe()
	store.EXPECT().ObjectExists(mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil).Maybe()
	store.EXPECT().DeleteObject(mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()

	storage := drive.NewStorage("d1", "my-bucket", nil, "us-east-1", "a", "s", false)
	drives := newDriveMock(t, storage)
	nodes := newNodeMock(t)
	path := newVfsMock(t)
	tm := newTxMock(t)

	svc := upload.NewService(upload.Config{
		TokenRegistry: reg,
		StorageLookup: drives,
		NodeLifecycle: nodes,
		ObjectStore:   store,
		Path:          path,
		TxManager:     tm,
	})
	_, err := svc.InitiateUpload(context.Background(), "u1", "d1", "/test.bin", nil, nil, time.Hour)
	assert.Error(t, err)
}

func TestCompleteUploadOwnershipMismatch(t *testing.T) {
	reg := upload.NewMemoryRegistry()
	_ = reg.Put(context.Background(), upload.PresignMeta{
		UploadID:  "u1",
		DriveID:   "d1",
		UserID:    "owner",
		Path:      "/x",
		Bucket:    "b",
		Key:       "k",
		ExpiresAt: time.Now().Add(time.Hour),
	}, time.Hour)
	svc := newTestService(t, reg)
	_, err := svc.CompleteUpload(context.Background(), "someone-else", "d1", "u1", 100, nil)
	assert.True(t, errorx.IsKind(err, errorx.KindPermissionDenied))
}

func TestCompleteUploadSizeMismatch(t *testing.T) {
	reg := upload.NewMemoryRegistry()
	size := int64(100)
	_ = reg.Put(context.Background(), upload.PresignMeta{
		UploadID:  "u1",
		DriveID:   "d1",
		UserID:    "user",
		Path:      "/x",
		Bucket:    "b",
		Key:       "k",
		Size:      &size,
		ExpiresAt: time.Now().Add(time.Hour),
	}, time.Hour)
	svc := newTestService(t, reg)
	_, err := svc.CompleteUpload(context.Background(), "user", "d1", "u1", 200, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "size mismatch")
}

func TestCompleteUploadObjectNotUploaded(t *testing.T) {
	reg := upload.NewMemoryRegistry()
	_ = reg.Put(context.Background(), upload.PresignMeta{
		UploadID:  "u1",
		DriveID:   "d1",
		UserID:    "user",
		Path:      "/x",
		Bucket:    "b",
		Key:       "k",
		ExpiresAt: time.Now().Add(time.Hour),
	}, time.Hour)

	store := uploadMocks.NewObjectStoreMock(t)
	store.EXPECT().GetPresignedUploadURL(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("url", nil).Maybe()
	store.EXPECT().GetPresignedDownloadURL(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("url", nil).Maybe()
	store.EXPECT().ObjectExists(mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()
	store.EXPECT().DeleteObject(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	storage := drive.NewStorage("d1", "my-bucket", nil, "us-east-1", "a", "s", false)
	drives := newDriveMock(t, storage)
	nodes := newNodeMock(t)
	path := newVfsMock(t)
	tm := newTxMock(t)

	svc := upload.NewService(upload.Config{
		TokenRegistry: reg,
		StorageLookup: drives,
		NodeLifecycle: nodes,
		ObjectStore:   store,
		Path:          path,
		TxManager:     tm,
	})
	_, err := svc.CompleteUpload(context.Background(), "user", "d1", "u1", 100, nil)
	assert.True(t, errorx.IsKind(err, errorx.KindNotFound))
}

func TestCompleteUploadHappyPath(t *testing.T) {
	reg := upload.NewMemoryRegistry()
	_ = reg.Put(context.Background(), upload.PresignMeta{
		UploadID:  "u1",
		DriveID:   "d1",
		UserID:    "user",
		Path:      "/x",
		Bucket:    "b",
		Key:       "k",
		ExpiresAt: time.Now().Add(time.Hour),
	}, time.Hour)
	svc := newTestService(t, reg)
	n, err := svc.CompleteUpload(context.Background(), "user", "d1", "u1", 100, nil)
	require.NoError(t, err)
	assert.NotNil(t, n)
	_, err = reg.Get(context.Background(), "u1")
	assert.Error(t, err)
}
