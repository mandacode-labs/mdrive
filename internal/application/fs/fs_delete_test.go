package fs

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
	inodeMocks "github.com/mandacode-labs/retrowin-go/internal/core/inode/mocks"
	objectMocks "github.com/mandacode-labs/retrowin-go/internal/core/object/mocks"
	userMocks "github.com/mandacode-labs/retrowin-go/internal/core/user/mocks"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func TestDelete_SuccessRegularFile(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	in := inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	inodeSvc.EXPECT().Delete(mock.Anything, "id-1").Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil, nil)

	err := svc.Delete(context.Background(), "id-1")

	assert.NoError(t, err)
}

func TestDelete_SuccessEmptyDirectory(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{}}
	raw, _ := json.Marshal(dirContent)
	in := inode.NewInode("id-1", "sys", inode.ModeDirectory|0755, 1000, 1000, 0, 1, 0, now, now, now, raw, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	inodeSvc.EXPECT().Delete(mock.Anything, "id-1").Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil, nil)

	err := svc.Delete(context.Background(), "id-1")

	assert.NoError(t, err)
}

func TestDelete_SuccessObjectInode(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	objectSvc := objectMocks.NewObjectServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	objContent := content.ObjectContent{ObjectID: "obj-1"}
	raw, _ := json.Marshal(objContent)
	in := inode.NewInode("id-1", "sys", inode.ModeObject|0644, 1000, 1000, 0, 1, 0, now, now, now, raw, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	objectSvc.EXPECT().Delete(mock.Anything, "obj-1").Return(nil)

	inodeSvc.EXPECT().Delete(mock.Anything, "id-1").Return(nil)

	svc := NewService(nil, inodeSvc, objectSvc, nil, userSvc, nil, nil)

	err := svc.Delete(context.Background(), "id-1")

	assert.NoError(t, err)
}

func TestDelete_ObjectNotFound(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	objectSvc := objectMocks.NewObjectServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	objContent := content.ObjectContent{ObjectID: "obj-1"}
	raw, _ := json.Marshal(objContent)
	in := inode.NewInode("id-1", "sys", inode.ModeObject|0644, 1000, 1000, 0, 1, 0, now, now, now, raw, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	objectSvc.EXPECT().Delete(mock.Anything, "obj-1").Return(errors.NotFound("object not found"))

	inodeSvc.EXPECT().Delete(mock.Anything, "id-1").Return(nil)

	svc := NewService(nil, inodeSvc, objectSvc, nil, userSvc, nil, nil)

	err := svc.Delete(context.Background(), "id-1")

	assert.NoError(t, err)
}

func TestDelete_ObjectDeleteError(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	objectSvc := objectMocks.NewObjectServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	objContent := content.ObjectContent{ObjectID: "obj-1"}
	raw, _ := json.Marshal(objContent)
	in := inode.NewInode("id-1", "sys", inode.ModeObject|0644, 1000, 1000, 0, 1, 0, now, now, now, raw, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	objectSvc.EXPECT().Delete(mock.Anything, "obj-1").Return(errors.Internal("delete failed"))

	inodeSvc.EXPECT().Delete(mock.Anything, "id-1").Return(nil)

	svc := NewService(nil, inodeSvc, objectSvc, nil, userSvc, nil, nil)

	err := svc.Delete(context.Background(), "id-1")

	assert.NoError(t, err)
}

func TestDelete_NonEmptyDirectory(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	in := inode.NewInode("id-1", "sys", inode.ModeDirectory|0755, 1000, 1000, 0, 1, 0, now, now, now, raw, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil, nil)

	err := svc.Delete(context.Background(), "id-1")

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestDelete_DirNilContent(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	in := inode.NewInode("id-1", "sys", inode.ModeDirectory|0755, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	inodeSvc.EXPECT().Delete(mock.Anything, "id-1").Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil, nil)

	err := svc.Delete(context.Background(), "id-1")

	assert.NoError(t, err)
}

func TestDelete_DirUnparsableContent(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	in := inode.NewInode("id-1", "sys", inode.ModeDirectory|0755, 1000, 1000, 0, 1, 0, now, now, now, []byte("invalid json"), now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	inodeSvc.EXPECT().Delete(mock.Anything, "id-1").Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil, nil)

	err := svc.Delete(context.Background(), "id-1")

	assert.NoError(t, err)
}

func TestDelete_NotFound(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)

	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(nil, errors.NotFound("inode not found"))

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil, nil)

	err := svc.Delete(context.Background(), "id-1")

	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestDelete_PermissionDenied(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	in := inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(2000, []int{2000}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil, nil)

	err := svc.Delete(context.Background(), "id-1")

	assert.Error(t, err)
	assert.True(t, errors.IsForbidden(err))
}

// --- deleteObjectRef ---

func TestDeleteObjectRef_Success(t *testing.T) {
	objectSvc := objectMocks.NewObjectServiceMock(t)

	objectSvc.EXPECT().Delete(mock.Anything, "obj-1").Return(nil)

	svc := NewService(nil, nil, objectSvc, nil, nil, nil, nil).(*service)

	now := time.Now()
	objContent := content.ObjectContent{ObjectID: "obj-1"}
	raw, _ := json.Marshal(objContent)
	in := inode.NewInode("test-id", "sys", inode.ModeObject|0644, 0, 0, 0, 1, 0, now, now, now, raw, now, now)

	err := svc.deleteObjectRef(context.Background(), in)

	assert.NoError(t, err)
}

func TestDeleteObjectRef_NotFound(t *testing.T) {
	objectSvc := objectMocks.NewObjectServiceMock(t)

	objectSvc.EXPECT().Delete(mock.Anything, "obj-1").Return(errors.NotFound("object not found"))

	svc := NewService(nil, nil, objectSvc, nil, nil, nil, nil).(*service)

	now := time.Now()
	objContent := content.ObjectContent{ObjectID: "obj-1"}
	raw, _ := json.Marshal(objContent)
	in := inode.NewInode("test-id", "sys", inode.ModeObject|0644, 0, 0, 0, 1, 0, now, now, now, raw, now, now)

	err := svc.deleteObjectRef(context.Background(), in)

	assert.NoError(t, err)
}

func TestDeleteObjectRef_OtherError(t *testing.T) {
	objectSvc := objectMocks.NewObjectServiceMock(t)

	objectSvc.EXPECT().Delete(mock.Anything, "obj-1").Return(errors.Internal("delete failed"))

	svc := NewService(nil, nil, objectSvc, nil, nil, nil, nil).(*service)

	now := time.Now()
	objContent := content.ObjectContent{ObjectID: "obj-1"}
	raw, _ := json.Marshal(objContent)
	in := inode.NewInode("test-id", "sys", inode.ModeObject|0644, 0, 0, 0, 1, 0, now, now, now, raw, now, now)

	err := svc.deleteObjectRef(context.Background(), in)

	assert.NoError(t, err)
}
