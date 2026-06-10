package vfs

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/mandacode-labs/mdrive/internal/core/inode"
	"github.com/mandacode-labs/mdrive/internal/core/inode/content"
	inodeMocks "github.com/mandacode-labs/mdrive/internal/core/inode/mocks"
	userMocks "github.com/mandacode-labs/mdrive/internal/core/user/mocks"
	"github.com/mandacode-labs/mdrive/internal/errors"
)

func TestUnlinkPath_Success(t *testing.T) {
	now := time.Now()
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	userSvc := userMocks.NewUserServiceMock(t)

	// Root inode with "file" entry
	dirContent := content.DirContent{
		Entries: []content.DirEntry{
			{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
		},
	}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|inode.PermOwnerRWX|inode.PermGroupRX|inode.PermOtherRX, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)

	// Mock Find for GetRootDirectory (called by ResolvePath)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	// Mock ReadDir for looking up entry in parent directory
	inodeSvc.EXPECT().ReadDir(mock.Anything, "root-id").Return([]inode.DirEntry{
		{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
	}, nil)

	// Mock Unlink
	inodeSvc.EXPECT().Unlink(mock.Anything, "root-id", "file").Return(nil)

	// Mock GetByID for Delete (permission check + actual delete)
	fileInode := inode.NewInode("file-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "file-id").Return(fileInode, nil)

	// Mock userSvc to return uid=0 (skips permission check)
	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(0, []int{}, nil)

	// Mock Delete
	inodeSvc.EXPECT().Delete(mock.Anything, "file-id").Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, NewLocker())

	err := svc.UnlinkPath(context.Background(), "sys", "/file")
	assert.NoError(t, err)
}

func TestUnlinkPath_NotFound(t *testing.T) {
	now := time.Now()
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)

	// Root inode with NO entries
	dirContent := content.DirContent{Entries: []content.DirEntry{}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|inode.PermOwnerRWX|inode.PermGroupRX|inode.PermOtherRX, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)

	// Mock Find for GetRootDirectory
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	// Mock ReadDir
	inodeSvc.EXPECT().ReadDir(mock.Anything, "root-id").Return([]inode.DirEntry{}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	err := svc.UnlinkPath(context.Background(), "sys", "/nonexistent")
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}
