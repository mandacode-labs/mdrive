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

func TestRm_SuccessSingleFile(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	// Root directory setup
	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)

	// ResolvePath calls Find to get root - use On for multiple calls
	inodeSvc.On("Find", mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	// ResolvePath calls GetByID for the file
	fileInode := inode.NewInode("file-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.On("GetByID", mock.Anything, "file-id").Return(fileInode, nil)

	// Permission check
	userSvc.On("ResolveUIDAndGIDs", mock.Anything, "sys").Return(1000, []int{1000}, nil)

	// Batch delete
	inodeSvc.On("Delete", mock.Anything, "file-id").Return(nil)

	// Batch unlink
	dentrySvc.On("UnlinkBatch", mock.Anything, "root-id", []string{"file"}).Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, dentrySvc, dentry.NewLocker())

	result, err := svc.Rm(context.Background(), &RmCommand{
		SystemID: "sys",
		Paths:    []string{"/file"},
	})

	assert.NoError(t, err)
	assert.Len(t, result.Deleted, 1)
	assert.Equal(t, "/file", result.Deleted[0])
	assert.Empty(t, result.Errors)
}

func TestRm_SuccessMultipleFiles(t *testing.T) {
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

	// ResolvePath for both files
	inodeSvc.On("Find", mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	file1Inode := inode.NewInode("file1-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.On("GetByID", mock.Anything, "file1-id").Return(file1Inode, nil)

	file2Inode := inode.NewInode("file2-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.On("GetByID", mock.Anything, "file2-id").Return(file2Inode, nil)

	// Permission checks
	userSvc.On("ResolveUIDAndGIDs", mock.Anything, "sys").Return(1000, []int{1000}, nil)

	// Batch delete
	inodeSvc.On("Delete", mock.Anything, "file1-id").Return(nil)
	inodeSvc.On("Delete", mock.Anything, "file2-id").Return(nil)

	// Batch unlink
	dentrySvc.On("UnlinkBatch", mock.Anything, "root-id", []string{"file1", "file2"}).Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, dentrySvc, dentry.NewLocker())

	result, err := svc.Rm(context.Background(), &RmCommand{
		SystemID: "sys",
		Paths:    []string{"/file1", "/file2"},
	})

	assert.NoError(t, err)
	assert.Len(t, result.Deleted, 2)
	assert.Contains(t, result.Deleted, "/file1")
	assert.Contains(t, result.Deleted, "/file2")
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

	// ResolvePath for /file1 and /nonexistent
	inodeSvc.On("Find", mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	file1Inode := inode.NewInode("file1-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.On("GetByID", mock.Anything, "file1-id").Return(file1Inode, nil)

	// Permission check for file1
	userSvc.On("ResolveUIDAndGIDs", mock.Anything, "sys").Return(1000, []int{1000}, nil)

	// Batch delete for file1
	inodeSvc.On("Delete", mock.Anything, "file1-id").Return(nil)

	// Batch unlink for file1
	dentrySvc.On("UnlinkBatch", mock.Anything, "root-id", []string{"file1"}).Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, dentrySvc, dentry.NewLocker())

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

	childInode := inode.NewInode("child-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)

	// ResolvePath for /dir
	inodeSvc.On("Find", mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode, childInode}, nil)

	dirInodeContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "child", InodeID: "child-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	dirRaw, _ := json.Marshal(dirInodeContent)
	dirInode := inode.NewInode("dir-id", "sys", inode.ModeDirectory|0755, 1000, 1000, 0, 1, 0, now, now, now, dirRaw, now, now)
	inodeSvc.On("GetByID", mock.Anything, "dir-id").Return(dirInode, nil)

	// Permission check for dir
	userSvc.On("ResolveUIDAndGIDs", mock.Anything, "sys").Return(1000, []int{1000}, nil)

	// For recursive delete, collect child entries from dir
	dentrySvc.On("ReadDir", mock.Anything, "dir-id").Return([]dentry.DirEntry{
		{Name: "child", InodeID: "child-id", FileType: uint8(inode.ModeRegular >> 12)},
	}, nil)

	inodeSvc.On("GetByID", mock.Anything, "child-id").Return(childInode, nil)

	// Batch delete for child and dir
	inodeSvc.On("Delete", mock.Anything, "child-id").Return(nil)
	inodeSvc.On("Delete", mock.Anything, "dir-id").Return(nil)

	// Batch unlink for child from dir and dir from root
	dentrySvc.On("UnlinkBatch", mock.Anything, "dir-id", []string{"child"}).Return(nil)
	dentrySvc.On("UnlinkBatch", mock.Anything, "root-id", []string{"dir"}).Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, dentrySvc, dentry.NewLocker())

	result, err := svc.Rm(context.Background(), &RmCommand{
		SystemID:  "sys",
		Paths:     []string{"/dir"},
		Recursive: true,
	})

	assert.NoError(t, err)
	assert.Len(t, result.Deleted, 1)
	assert.Equal(t, "/dir", result.Deleted[0])
	assert.Empty(t, result.Errors)
}

func TestRm_PermissionDenied(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)

	// ResolvePath
	inodeSvc.On("Find", mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)
	fileInode := inode.NewInode("file-id", "sys", inode.ModeRegular|0644, 2000, 2000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.On("GetByID", mock.Anything, "file-id").Return(fileInode, nil)

	// Permission check fails
	userSvc.On("ResolveUIDAndGIDs", mock.Anything, "sys").Return(1000, []int{1000}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, dentrySvc, dentry.NewLocker())

	result, err := svc.Rm(context.Background(), &RmCommand{
		SystemID: "sys",
		Paths:    []string{"/file"},
	})

	assert.NoError(t, err)
	assert.Empty(t, result.Deleted)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "/file", result.Errors[0].Path)
	assert.True(t, errors.IsForbidden(result.Errors[0].Error))
}
