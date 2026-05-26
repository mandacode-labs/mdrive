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
	userMocks "github.com/mandacode-labs/retrowin-go/internal/core/user/mocks"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func TestChmodPath_InvalidMode(t *testing.T) {
	svc := NewService(nil, nil, nil, nil)

	t.Run("negative mode", func(t *testing.T) {
		_, err := svc.ChmodPath(context.Background(), "sys", "/path", -1)
		assert.Error(t, err)
		assert.True(t, errors.IsBadRequest(err))
	})

	t.Run("mode exceeds 0o777", func(t *testing.T) {
		_, err := svc.ChmodPath(context.Background(), "sys", "/path", 0o1000)
		assert.Error(t, err)
		assert.True(t, errors.IsBadRequest(err))
	})
}

func TestChmodPath_Success(t *testing.T) {
	now := time.Now()
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)

	// Root inode with "file" entry in directory content
	dirContent := content.DirContent{
		Entries: []content.DirEntry{
			{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
		},
	}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|inode.PermOwnerRWX|inode.PermGroupRX|inode.PermOtherRX, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)

	// Mock Find for GetRootDirectory (called by ResolvePath)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	// Mock GetByID for the file inode (called by ResolvePath when traversing)
	fileInode := inode.NewInode("file-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "file-id").Return(fileInode, nil)

	// Mock GetByID for permission check (called by UpdateMode)
	inodeSvc.EXPECT().GetByID(mock.Anything, "file-id").Return(fileInode, nil)

	// Mock userSvc to return uid=0 (skips permission check)
	// Called by both UpdateMode and Get
	userSvc.On("ResolveUIDAndGIDs", mock.Anything, "sys").Return(0, []int{}, nil)

	// Mock Update (called by UpdateMode)
	inodeSvc.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)

	// Mock GetByID for returning updated inode (called by Get)
	updatedInode := inode.NewInode("file-id", "sys", inode.ModeRegular|0755, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "file-id").Return(updatedInode, nil)

	svc := NewService(inodeSvc, nil, userSvc, nil)

	result, err := svc.ChmodPath(context.Background(), "sys", "/file", 0755)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestChmodPath_NotFound(t *testing.T) {
	now := time.Now()
	inodeSvc := inodeMocks.NewInodeServiceMock(t)

	// Root inode with NO entries
	dirContent := content.DirContent{Entries: []content.DirEntry{}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|inode.PermOwnerRWX|inode.PermGroupRX|inode.PermOtherRX, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)

	// Mock Find for GetRootDirectory
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	svc := NewService(inodeSvc, nil, nil, nil)

	_, err := svc.ChmodPath(context.Background(), "sys", "/nonexistent", 0755)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}
