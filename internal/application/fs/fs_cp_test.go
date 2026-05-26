package fs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	inodeMocks "github.com/mandacode-labs/retrowin-go/internal/core/inode/mocks"
	userMocks "github.com/mandacode-labs/retrowin-go/internal/core/user/mocks"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func TestCopy_Success(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	original := inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 1024, 1, 0, now, now, now, []byte("content"), now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(original, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	userSvc.EXPECT().ResolveUID(mock.Anything, "sys2").Return(2000, nil)

	copied := inode.NewInode("id-2", "sys2", inode.ModeRegular|0644, 2000, 1000, 1024, 1, 0, now, now, now, []byte("content"), now, now)
	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(copied, nil)

	svc := NewService(inodeSvc, nil, userSvc, nil)

	result, err := svc.Copy(context.Background(), "id-1", "sys2")

	assert.NoError(t, err)
	assert.Equal(t, copied, result)
}

func TestCopy_ObjectInode(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	original := inode.NewInode("id-1", "sys", inode.ModeObject|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(original, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	svc := NewService(inodeSvc, nil, userSvc, nil)

	_, err := svc.Copy(context.Background(), "id-1", "sys2")

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestCopy_NotFound(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)

	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(nil, errors.NotFound("inode not found"))

	svc := NewService(inodeSvc, nil, nil, nil)

	_, err := svc.Copy(context.Background(), "id-1", "sys2")

	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestCopy_PermissionDenied(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	original := inode.NewInode("id-1", "sys", inode.ModeRegular|0600, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(original, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(2000, []int{2000}, nil)

	svc := NewService(inodeSvc, nil, userSvc, nil)

	_, err := svc.Copy(context.Background(), "id-1", "sys2")

	assert.Error(t, err)
	assert.True(t, errors.IsForbidden(err))
}

func TestCopy_ResolveUIDFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	original := inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(original, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	userSvc.EXPECT().ResolveUID(mock.Anything, "sys2").Return(0, errors.Internal("resolve failed"))

	svc := NewService(inodeSvc, nil, userSvc, nil)

	_, err := svc.Copy(context.Background(), "id-1", "sys2")

	assert.Error(t, err)
}

func TestCopy_CreateFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	original := inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(original, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	userSvc.EXPECT().ResolveUID(mock.Anything, "sys2").Return(2000, nil)

	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, errors.Internal("create failed"))

	svc := NewService(inodeSvc, nil, userSvc, nil)

	_, err := svc.Copy(context.Background(), "id-1", "sys2")

	assert.Error(t, err)
}
