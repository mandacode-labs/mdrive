package vfs

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

func TestUpdateContent_Success(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	in := inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	content := []byte("hello world")
	var capturedCmd *inode.UpdateCommand
	inodeSvc.EXPECT().Update(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, cmd *inode.UpdateCommand) error {
		capturedCmd = cmd
		return nil
	})

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil)

	result, err := svc.UpdateContent(context.Background(), &UpdateContentCommand{
		ID:      "id-1",
		Content: content,
	})

	assert.NoError(t, err)
	assert.Equal(t, in, result)
	assert.NotNil(t, capturedCmd)
	assert.Equal(t, "id-1", capturedCmd.ID)
	assert.NotNil(t, capturedCmd.Size)
	assert.Equal(t, int64(len(content)), *capturedCmd.Size)
}

func TestUpdateContent_NotFound(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)

	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(nil, errors.NotFound("inode not found"))

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	_, err := svc.UpdateContent(context.Background(), &UpdateContentCommand{
		ID:      "id-1",
		Content: []byte("test"),
	})

	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestUpdateContent_PermissionDenied(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	in := inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(2000, []int{2000}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil)

	_, err := svc.UpdateContent(context.Background(), &UpdateContentCommand{
		ID:      "id-1",
		Content: []byte("test"),
	})

	assert.Error(t, err)
	assert.True(t, errors.IsForbidden(err))
}

func TestUpdateContent_UpdateFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	in := inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	inodeSvc.EXPECT().Update(mock.Anything, mock.Anything).Return(errors.Internal("update failed"))

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil)

	_, err := svc.UpdateContent(context.Background(), &UpdateContentCommand{
		ID:      "id-1",
		Content: []byte("test"),
	})

	assert.Error(t, err)
}

// --- UpdateMode ---

func TestUpdateMode_Success(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	in := inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	var capturedCmd *inode.UpdateCommand
	inodeSvc.EXPECT().Update(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, cmd *inode.UpdateCommand) error {
		capturedCmd = cmd
		return nil
	})

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil)

	err := svc.UpdateMode(context.Background(), &UpdateModeCommand{
		ID:   "id-1",
		Mode: 0755,
	})

	assert.NoError(t, err)
	assert.NotNil(t, capturedCmd)
	assert.Equal(t, "id-1", capturedCmd.ID)
	assert.NotNil(t, capturedCmd.Mode)
	assert.Equal(t, 0755, *capturedCmd.Mode)
}

func TestUpdateMode_NotFound(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)

	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(nil, errors.NotFound("inode not found"))

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	err := svc.UpdateMode(context.Background(), &UpdateModeCommand{
		ID:   "id-1",
		Mode: 0755,
	})

	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestUpdateMode_PermissionDenied(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	in := inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(2000, []int{2000}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil)

	err := svc.UpdateMode(context.Background(), &UpdateModeCommand{
		ID:   "id-1",
		Mode: 0755,
	})

	assert.Error(t, err)
	assert.True(t, errors.IsForbidden(err))
}

func TestUpdateMode_UpdateFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	in := inode.NewInode("id-1", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "id-1").Return(in, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	inodeSvc.EXPECT().Update(mock.Anything, mock.Anything).Return(errors.Internal("update failed"))

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, nil)

	err := svc.UpdateMode(context.Background(), &UpdateModeCommand{
		ID:   "id-1",
		Mode: 0755,
	})

	assert.Error(t, err)
}
