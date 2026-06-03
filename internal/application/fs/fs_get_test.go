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

func TestGet_Success(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	in := inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil, nil)

	result, err := svc.Get(context.Background(), "id-1")

	assert.NoError(t, err)
	assert.Equal(t, in, result)
}

func TestGet_NotFound(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)

	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(nil, errors.NotFound("inode not found"))

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil, nil)

	_, err := svc.Get(context.Background(), "id-1")

	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestGet_PermissionDenied(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	in := inode.NewInode("id-1", "sys", inode.ModeRegular|0600, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(2000, []int{2000}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil, nil)

	_, err := svc.Get(context.Background(), "id-1")

	assert.Error(t, err)
	assert.True(t, errors.IsForbidden(err))
}

func TestGet_RootBypass(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	in := inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(0, []int{}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil, nil)

	result, err := svc.Get(context.Background(), "id-1")

	assert.NoError(t, err)
	assert.Equal(t, in, result)
}

// --- List ---

func TestList_WithSystemID(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	now := time.Now()

	sysID := "sys"
	inodes := []*inode.Inode{
		inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now),
	}
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return(inodes, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil, nil)

	result, err := svc.List(context.Background(), &ListFilter{
		SystemID: &sysID,
	})

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestList_WithUID(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	now := time.Now()

	uid := 1000
	inodes := []*inode.Inode{
		inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now),
	}
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return(inodes, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil, nil)

	result, err := svc.List(context.Background(), &ListFilter{
		UID: &uid,
	})

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestList_WithBothFilters(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	now := time.Now()

	sysID := "sys"
	uid := 1000
	inodes := []*inode.Inode{
		inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now),
	}
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return(inodes, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil, nil)

	result, err := svc.List(context.Background(), &ListFilter{
		SystemID: &sysID,
		UID:      &uid,
	})

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestList_EmptyFilter(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)

	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil, nil)

	result, err := svc.List(context.Background(), &ListFilter{})

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestList_FindFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)

	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return(nil, errors.Internal("find failed"))

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil, nil)

	_, err := svc.List(context.Background(), &ListFilter{})

	assert.Error(t, err)
}
