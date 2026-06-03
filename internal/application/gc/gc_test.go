package gc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/retrowin-go/internal/core/object"
	objectMocks "github.com/mandacode-labs/retrowin-go/internal/core/object/mocks"
)

func newTestObject(id, bucket, storageKey string, status object.Status) *object.Object {
	return object.NewObject(id, object.ProviderS3, bucket, "system-1", storageKey, status, nil, nil, time.Now(), time.Now())
}

func TestGC_DefaultExpiry(t *testing.T) {
	gc := NewGarbageCollector(nil, nil, 0)
	assert.Equal(t, DefaultPendingExpiry, gc.expiry)
}

func TestGC_CustomExpiry(t *testing.T) {
	custom := 48 * time.Hour
	gc := NewGarbageCollector(nil, nil, custom)
	assert.Equal(t, custom, gc.expiry)
}

func TestGC_Run_NoObjects(t *testing.T) {
	objSvc := objectMocks.NewObjectServiceMock(t)
	storageMock := objectMocks.NewStorageMock(t)

	objSvc.EXPECT().FindPendingOlderThan(context.Background(), DefaultPendingExpiry).Return(nil, nil)
	objSvc.EXPECT().FindActive(context.Background()).Return(nil, nil)

	gc := NewGarbageCollector(objSvc, storageMock, 0)
	result, err := gc.Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 0, result.PendingCleaned)
	assert.Equal(t, 0, result.OrphansCleaned)
}

func TestGC_Run_PendingCleanup(t *testing.T) {
	objSvc := objectMocks.NewObjectServiceMock(t)
	storageMock := objectMocks.NewStorageMock(t)

	pending := []*object.Object{
		newTestObject("pending-1", "bucket", "key-1", object.StatusPending),
		newTestObject("pending-2", "bucket", "key-2", object.StatusPending),
	}

	objSvc.EXPECT().FindPendingOlderThan(context.Background(), DefaultPendingExpiry).Return(pending, nil)
	objSvc.EXPECT().Delete(context.Background(), "pending-1").Return(nil)
	objSvc.EXPECT().Delete(context.Background(), "pending-2").Return(nil)
	objSvc.EXPECT().FindActive(context.Background()).Return(nil, nil)

	gc := NewGarbageCollector(objSvc, storageMock, 0)
	result, err := gc.Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, result.PendingCleaned)
	assert.Equal(t, 0, result.OrphansCleaned)
}

func TestGC_Run_PartialPendingFailure(t *testing.T) {
	objSvc := objectMocks.NewObjectServiceMock(t)
	storageMock := objectMocks.NewStorageMock(t)

	pending := []*object.Object{
		newTestObject("pending-1", "bucket", "key-1", object.StatusPending),
		newTestObject("pending-2", "bucket", "key-2", object.StatusPending),
	}

	objSvc.EXPECT().FindPendingOlderThan(context.Background(), DefaultPendingExpiry).Return(pending, nil)
	objSvc.EXPECT().Delete(context.Background(), "pending-1").Return(nil)
	objSvc.EXPECT().Delete(context.Background(), "pending-2").Return(errors.New("db error"))
	objSvc.EXPECT().FindActive(context.Background()).Return(nil, nil)

	gc := NewGarbageCollector(objSvc, storageMock, 0)
	result, err := gc.Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, result.PendingCleaned)
}

func TestGC_Run_OrphanCleanup(t *testing.T) {
	objSvc := objectMocks.NewObjectServiceMock(t)
	storageMock := objectMocks.NewStorageMock(t)

	active := []*object.Object{
		newTestObject("active-1", "bucket", "key-1", object.StatusActive),
		newTestObject("active-2", "bucket", "key-2", object.StatusActive),
	}

	objSvc.EXPECT().FindPendingOlderThan(context.Background(), DefaultPendingExpiry).Return(nil, nil)
	objSvc.EXPECT().FindActive(context.Background()).Return(active, nil)

	// First object exists in S3, second is missing (orphan)
	storageMock.EXPECT().ObjectExists(context.Background(), "bucket", "key-1").Return(true, nil)
	storageMock.EXPECT().ObjectExists(context.Background(), "bucket", "key-2").Return(false, nil)

	objSvc.EXPECT().DeleteFromDB(context.Background(), "active-2").Return(nil)

	gc := NewGarbageCollector(objSvc, storageMock, 0)
	result, err := gc.Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 0, result.PendingCleaned)
	assert.Equal(t, 1, result.OrphansCleaned)
}

func TestGC_Run_PartialOrphanFailure(t *testing.T) {
	objSvc := objectMocks.NewObjectServiceMock(t)
	storageMock := objectMocks.NewStorageMock(t)

	active := []*object.Object{
		newTestObject("active-1", "bucket", "key-1", object.StatusActive),
		newTestObject("active-2", "bucket", "key-2", object.StatusActive),
	}

	objSvc.EXPECT().FindPendingOlderThan(context.Background(), DefaultPendingExpiry).Return(nil, nil)
	objSvc.EXPECT().FindActive(context.Background()).Return(active, nil)

	// Both missing from S3
	storageMock.EXPECT().ObjectExists(context.Background(), "bucket", "key-1").Return(false, nil)
	storageMock.EXPECT().ObjectExists(context.Background(), "bucket", "key-2").Return(false, nil)

	// First delete fails, second succeeds
	objSvc.EXPECT().DeleteFromDB(context.Background(), "active-1").Return(errors.New("db error"))
	objSvc.EXPECT().DeleteFromDB(context.Background(), "active-2").Return(nil)

	gc := NewGarbageCollector(objSvc, storageMock, 0)
	result, err := gc.Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, result.OrphansCleaned)
}

func TestGC_Run_ObjectExistsCheckFailure(t *testing.T) {
	objSvc := objectMocks.NewObjectServiceMock(t)
	storageMock := objectMocks.NewStorageMock(t)

	active := []*object.Object{
		newTestObject("active-1", "bucket", "key-1", object.StatusActive),
	}

	objSvc.EXPECT().FindPendingOlderThan(context.Background(), DefaultPendingExpiry).Return(nil, nil)
	objSvc.EXPECT().FindActive(context.Background()).Return(active, nil)

	// ObjectExists returns error — should skip, not delete
	storageMock.EXPECT().ObjectExists(context.Background(), "bucket", "key-1").Return(false, errors.New("s3 error"))

	gc := NewGarbageCollector(objSvc, storageMock, 0)
	result, err := gc.Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 0, result.OrphansCleaned)
}

func TestGC_Run_FindPendingError(t *testing.T) {
	objSvc := objectMocks.NewObjectServiceMock(t)
	storageMock := objectMocks.NewStorageMock(t)

	objSvc.EXPECT().FindPendingOlderThan(context.Background(), DefaultPendingExpiry).Return(nil, errors.New("db error"))

	gc := NewGarbageCollector(objSvc, storageMock, 0)
	_, err := gc.Run(context.Background())

	require.Error(t, err)
}

func TestGC_Run_FindActiveError(t *testing.T) {
	objSvc := objectMocks.NewObjectServiceMock(t)
	storageMock := objectMocks.NewStorageMock(t)

	objSvc.EXPECT().FindPendingOlderThan(context.Background(), DefaultPendingExpiry).Return(nil, nil)
	objSvc.EXPECT().FindActive(context.Background()).Return(nil, errors.New("db error"))

	gc := NewGarbageCollector(objSvc, storageMock, 0)
	_, err := gc.Run(context.Background())

	require.Error(t, err)
}
