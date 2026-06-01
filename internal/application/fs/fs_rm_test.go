package fs

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/mandacode-labs/retrowin-go/internal/core/dentry"
	dentryMocks "github.com/mandacode-labs/retrowin-go/internal/core/dentry/mocks"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
	inodeMocks "github.com/mandacode-labs/retrowin-go/internal/core/inode/mocks"
	userMocks "github.com/mandacode-labs/retrowin-go/internal/core/user/mocks"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func TestRm_EmptyPaths(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.Rm(context.Background(), &RmCommand{
		SystemID: "sys",
		Paths:    []string{},
	})

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestRm_SuccessSinglePath(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	fileInode := inode.NewInode("file-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "file-id").Return(fileInode, nil)

	dentrySvc.EXPECT().ReadDir(mock.Anything, "root-id").Return([]dentry.DirEntry{{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)}}, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	dentrySvc.EXPECT().Unlink(mock.Anything, "root-id", "file").Return(nil)

	inodeSvc.EXPECT().Delete(mock.Anything, "file-id").Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, dentrySvc, nil)

	result, err := svc.Rm(context.Background(), &RmCommand{
		SystemID: "sys",
		Paths:    []string{"/file"},
	})

	assert.NoError(t, err)
	assert.Len(t, result.Deleted, 1)
	assert.Equal(t, "/file", result.Deleted[0])
	assert.Empty(t, result.Errors)
}

func TestRm_SuccessMultiplePaths(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "file1", InodeID: "file1-id", FileType: uint8(inode.ModeRegular >> 12)},
		{Name: "file2", InodeID: "file2-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	file1Inode := inode.NewInode("file1-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "file1-id").Return(file1Inode, nil)

	userSvc.On("ResolveUIDAndGIDs", mock.Anything, "sys").Return(1000, []int{1000}, nil)

	dentrySvc.On("ReadDir", mock.Anything, "root-id").Return([]dentry.DirEntry{
		{Name: "file1", InodeID: "file1-id", FileType: uint8(inode.ModeRegular >> 12)},
		{Name: "file2", InodeID: "file2-id", FileType: uint8(inode.ModeRegular >> 12)},
	}, nil)

	dentrySvc.EXPECT().Unlink(mock.Anything, "root-id", "file1").Return(nil)

	inodeSvc.EXPECT().Delete(mock.Anything, "file1-id").Return(nil)

	file2Inode := inode.NewInode("file2-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "file2-id").Return(file2Inode, nil)

	dentrySvc.EXPECT().Unlink(mock.Anything, "root-id", "file2").Return(nil)

	inodeSvc.EXPECT().Delete(mock.Anything, "file2-id").Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, dentrySvc, nil)

	result, err := svc.Rm(context.Background(), &RmCommand{
		SystemID: "sys",
		Paths:    []string{"/file1", "/file2"},
	})

	assert.NoError(t, err)
	assert.Len(t, result.Deleted, 2)
	assert.Empty(t, result.Errors)
}

func TestRm_MixedSuccessFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "file1", InodeID: "file1-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	file1Inode := inode.NewInode("file1-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "file1-id").Return(file1Inode, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	dentrySvc.On("ReadDir", mock.Anything, "root-id").Return([]dentry.DirEntry{{Name: "file1", InodeID: "file1-id", FileType: uint8(inode.ModeRegular >> 12)}}, nil)

	dentrySvc.EXPECT().Unlink(mock.Anything, "root-id", "file1").Return(nil)

	inodeSvc.EXPECT().Delete(mock.Anything, "file1-id").Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, dentrySvc, nil)

	result, err := svc.Rm(context.Background(), &RmCommand{
		SystemID: "sys",
		Paths:    []string{"/file1", "/nonexistent"},
	})

	assert.NoError(t, err)
	assert.Len(t, result.Deleted, 1)
	assert.Equal(t, "/file1", result.Deleted[0])
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "/nonexistent", result.Errors[0].Path)
}

func TestRm_NonEmptyDirectory(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "dir", InodeID: "dir-id", FileType: uint8(inode.ModeDirectory >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	dirInodeContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "child", InodeID: "child-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	dirRaw, _ := json.Marshal(dirInodeContent)
	dirInode := inode.NewInode("dir-id", "sys", inode.ModeDirectory|0755, 1000, 1000, 0, 1, 0, now, now, now, dirRaw, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "dir-id").Return(dirInode, nil)

	dentrySvc.EXPECT().ReadDir(mock.Anything, "root-id").Return([]dentry.DirEntry{{Name: "dir", InodeID: "dir-id", FileType: uint8(inode.ModeDirectory >> 12)}}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, dentrySvc, nil)

	result, err := svc.Rm(context.Background(), &RmCommand{
		SystemID: "sys",
		Paths:    []string{"/dir"},
	})

	assert.NoError(t, err)
	assert.Empty(t, result.Deleted)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "/dir", result.Errors[0].Path)
	assert.True(t, errors.IsBadRequest(result.Errors[0].Error))
}

func TestRm_UnlinkFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	dentrySvc.EXPECT().ReadDir(mock.Anything, "root-id").Return([]dentry.DirEntry{{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)}}, nil)

	dentrySvc.EXPECT().Unlink(mock.Anything, "root-id", "file").Return(errors.Internal("unlink failed"))

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, dentrySvc, nil)

	result, err := svc.Rm(context.Background(), &RmCommand{
		SystemID: "sys",
		Paths:    []string{"/file"},
	})

	assert.NoError(t, err)
	assert.Empty(t, result.Deleted)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "/file", result.Errors[0].Path)
}

func TestRm_DeleteFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	fileInode := inode.NewInode("file-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "file-id").Return(fileInode, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	dentrySvc.EXPECT().ReadDir(mock.Anything, "root-id").Return([]dentry.DirEntry{{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)}}, nil)

	dentrySvc.EXPECT().Unlink(mock.Anything, "root-id", "file").Return(nil)

	inodeSvc.EXPECT().Delete(mock.Anything, "file-id").Return(errors.Internal("delete failed"))

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, dentrySvc, nil)

	result, err := svc.Rm(context.Background(), &RmCommand{
		SystemID: "sys",
		Paths:    []string{"/file"},
	})

	assert.NoError(t, err)
	assert.Empty(t, result.Deleted)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "/file", result.Errors[0].Path)
}

// --- rmOne ---

func TestRmOne_NotFound(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	dentrySvc.EXPECT().ReadDir(mock.Anything, "root-id").Return([]dentry.DirEntry{}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, dentrySvc, nil).(*service)

	err := svc.rmOne(context.Background(), "sys", "/nonexistent", false)

	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}
