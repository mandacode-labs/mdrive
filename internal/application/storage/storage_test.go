package storage

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/retrowin-go/internal/application/vfs"
	vfsMocks "github.com/mandacode-labs/retrowin-go/internal/application/vfs/mocks"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
	"github.com/mandacode-labs/retrowin-go/internal/core/object"
	objectMocks "github.com/mandacode-labs/retrowin-go/internal/core/object/mocks"
)

func newTestObject(id string) *object.Object {
	return object.NewObject(id, object.ProviderS3, "bucket", "system-1", "key-"+id, object.StatusActive, nil, nil, time.Now(), time.Now())
}

func TestCompleteUpload_PassSizeAndModeToInode(t *testing.T) {
	ctx := context.Background()
	objSvc := objectMocks.NewObjectServiceMock(t)
	fsSvc := vfsMocks.NewVFSMock(t)

	obj := newTestObject("obj-1")
	mode := inode.ModeObject | 0644

	objSvc.EXPECT().CompleteUpload(ctx, "obj-1").Return(obj, nil)
	objSvc.EXPECT().GetObjectSize(ctx, "obj-1").Return(int64(4096), nil)

	fsSvc.EXPECT().CreateFile(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, cmd *vfs.CreateFileCommand) (*inode.Inode, error) {
		assert.Equal(t, int64(4096), cmd.Size)
		assert.Equal(t, mode, cmd.Mode)
		assert.Equal(t, "system-1", cmd.SystemID)

		// Verify ObjectContent is in the content
		var objContent content.ObjectContent
		require.NoError(t, json.Unmarshal(cmd.Content, &objContent))
		assert.Equal(t, "obj-1", objContent.ObjectID)

		return inode.NewInode("inode-1", cmd.SystemID, cmd.Mode, 1000, 1000, cmd.Size, 1, 0, time.Now(), time.Now(), time.Now(), cmd.Content, time.Now(), time.Now()), nil
	})
	objSvc.EXPECT().GetByID(ctx, "obj-1").Return(obj, nil)

	svc := NewService(fsSvc, objSvc)
	result, err := svc.CompleteUpload(ctx, &CompleteUploadCommand{
		ObjectID: "obj-1",
		SystemID: "system-1",
		Mode:     mode,
	})

	require.NoError(t, err)
	require.NotNil(t, result.Inode)
	assert.Equal(t, int64(4096), result.Inode.Size())
	assert.Equal(t, mode, result.Inode.Mode())
}

func TestCompleteUpload_DefaultMode(t *testing.T) {
	ctx := context.Background()
	objSvc := objectMocks.NewObjectServiceMock(t)
	fsSvc := vfsMocks.NewVFSMock(t)

	obj := newTestObject("obj-2")
	expectedDefault := inode.ModeObject | inode.PermOwnerRW | inode.PermGroupRX | inode.PermOtherR

	objSvc.EXPECT().CompleteUpload(ctx, "obj-2").Return(obj, nil)
	objSvc.EXPECT().GetObjectSize(ctx, "obj-2").Return(int64(0), nil)

	fsSvc.EXPECT().CreateFile(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, cmd *vfs.CreateFileCommand) (*inode.Inode, error) {
		assert.Equal(t, expectedDefault, cmd.Mode)
		return inode.NewInode("inode-2", cmd.SystemID, cmd.Mode, 1000, 1000, cmd.Size, 1, 0, time.Now(), time.Now(), time.Now(), cmd.Content, time.Now(), time.Now()), nil
	})
	objSvc.EXPECT().GetByID(ctx, "obj-2").Return(obj, nil)

	svc := NewService(fsSvc, objSvc)
	result, err := svc.CompleteUpload(ctx, &CompleteUploadCommand{
		ObjectID: "obj-2",
		SystemID: "system-1",
		Mode:     0,
	})

	require.NoError(t, err)
	assert.Equal(t, expectedDefault, result.Inode.Mode())
}

func TestCompleteUpload_GetObjectSizeError(t *testing.T) {
	ctx := context.Background()
	objSvc := objectMocks.NewObjectServiceMock(t)
	fsSvc := vfsMocks.NewVFSMock(t)

	obj := newTestObject("obj-3")

	objSvc.EXPECT().CompleteUpload(ctx, "obj-3").Return(obj, nil)
	objSvc.EXPECT().GetObjectSize(ctx, "obj-3").Return(int64(0), assert.AnError)

	svc := NewService(fsSvc, objSvc)
	_, err := svc.CompleteUpload(ctx, &CompleteUploadCommand{
		ObjectID: "obj-3",
		SystemID: "system-1",
	})

	require.Error(t, err)
}

func TestCompleteUpload_CompleteUploadError(t *testing.T) {
	ctx := context.Background()
	objSvc := objectMocks.NewObjectServiceMock(t)
	fsSvc := vfsMocks.NewVFSMock(t)

	objSvc.EXPECT().CompleteUpload(ctx, "obj-missing").Return(nil, assert.AnError)

	svc := NewService(fsSvc, objSvc)
	_, err := svc.CompleteUpload(ctx, &CompleteUploadCommand{
		ObjectID: "obj-missing",
		SystemID: "system-1",
	})

	require.Error(t, err)
}

func TestCompleteUpload_MissingObjectID(t *testing.T) {
	svc := NewService(nil, nil)
	_, err := svc.CompleteUpload(context.Background(), &CompleteUploadCommand{
		SystemID: "system-1",
	})
	require.Error(t, err)
}

func TestCompleteUpload_MissingSystemID(t *testing.T) {
	svc := NewService(nil, nil)
	_, err := svc.CompleteUpload(context.Background(), &CompleteUploadCommand{
		ObjectID: "obj-1",
	})
	require.Error(t, err)
}
