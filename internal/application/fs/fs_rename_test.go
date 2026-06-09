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

func TestRename_EmptyNewName(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.Rename(context.Background(), &RenameCommand{
		SystemID: "sys",
		Path:     "/file",
		NewName:  "",
	})

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestRename_NewNameWithPathSeparator(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.Rename(context.Background(), &RenameCommand{
		SystemID: "sys",
		Path:     "/file",
		NewName:  "dir/file",
	})

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestRename_Success(t *testing.T) {
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

	dentrySvc.EXPECT().Link(mock.Anything, "root-id", mock.Anything).Return(nil)

	dentrySvc.EXPECT().Unlink(mock.Anything, "root-id", "file").Return(nil)

	inodeSvc.EXPECT().GetByID(mock.Anything, "file-id").Return(fileInode, nil)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, userSvc, dentrySvc, dentry.NewLocker())

	result, err := svc.Rename(context.Background(), &RenameCommand{
		SystemID: "sys",
		Path:     "/file",
		NewName:  "newfile",
	})

	assert.NoError(t, err)
	assert.Equal(t, fileInode, result)
}

func TestRename_SourceNotFound(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil, nil)

	_, err := svc.Rename(context.Background(), &RenameCommand{
		SystemID: "sys",
		Path:     "/nonexistent",
		NewName:  "newfile",
	})

	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestRename_TargetExists(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
		{Name: "newfile", InodeID: "newfile-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	fileInode := inode.NewInode("file-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "file-id").Return(fileInode, nil)

	dentrySvc.EXPECT().ReadDir(mock.Anything, "root-id").Return([]dentry.DirEntry{
		{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
		{Name: "newfile", InodeID: "newfile-id", FileType: uint8(inode.ModeRegular >> 12)},
	}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, dentrySvc, nil)

	_, err := svc.Rename(context.Background(), &RenameCommand{
		SystemID: "sys",
		Path:     "/file",
		NewName:  "newfile",
	})

	assert.Error(t, err)
	assert.True(t, errors.IsConflict(err))
}

func TestRename_LinkFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
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

	dentrySvc.EXPECT().Link(mock.Anything, "root-id", mock.Anything).Return(errors.Internal("link failed"))

	svc := NewService(nil, inodeSvc, nil, nil, nil, dentrySvc, nil)

	_, err := svc.Rename(context.Background(), &RenameCommand{
		SystemID: "sys",
		Path:     "/file",
		NewName:  "newfile",
	})

	assert.Error(t, err)
}

func TestRename_UnlinkFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
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

	dentrySvc.EXPECT().Link(mock.Anything, "root-id", mock.Anything).Return(nil)

	dentrySvc.EXPECT().Unlink(mock.Anything, "root-id", "file").Return(errors.Internal("unlink failed"))

	svc := NewService(nil, inodeSvc, nil, nil, nil, dentrySvc, nil)

	_, err := svc.Rename(context.Background(), &RenameCommand{
		SystemID: "sys",
		Path:     "/file",
		NewName:  "newfile",
	})

	assert.Error(t, err)
}
