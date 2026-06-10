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
	"github.com/mandacode-labs/mdrive/internal/errors"
)

func TestMv_EmptySources(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil)

	_, err := svc.Mv(context.Background(), &MvCommand{
		SystemID:    "sys",
		Sources:     []string{},
		Destination: "/dest",
	})

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestMv_EmptyDestination(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil)

	_, err := svc.Mv(context.Background(), &MvCommand{
		SystemID:    "sys",
		Sources:     []string{"/src"},
		Destination: "",
	})

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestMv_SuccessMoveToDirectory(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	// Root with src file and dest dir
	rootContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "src", InodeID: "src-id", FileType: uint8(inode.ModeRegular >> 12)},
		{Name: "dest", InodeID: "dest-id", FileType: uint8(inode.ModeDirectory >> 12)},
	}}
	raw, _ := json.Marshal(rootContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	srcInode := inode.NewInode("src-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "src-id").Return(srcInode, nil)

	destContent := content.DirContent{Entries: []content.DirEntry{}}
	destRaw, _ := json.Marshal(destContent)
	destInode := inode.NewInode("dest-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, destRaw, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "dest-id").Return(destInode, nil)

	inodeSvc.EXPECT().ReadDir(mock.Anything, "dest-id").Return([]inode.DirEntry{}, nil)

	inodeSvc.EXPECT().Link(mock.Anything, "dest-id", mock.Anything).Return(nil)

	inodeSvc.EXPECT().Unlink(mock.Anything, "root-id", "src").Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	result, err := svc.Mv(context.Background(), &MvCommand{
		SystemID:    "sys",
		Sources:     []string{"/src"},
		Destination: "/dest",
	})

	assert.NoError(t, err)
	assert.Len(t, result.Moved, 1)
	assert.Equal(t, "/src", result.Moved[0])
	assert.Empty(t, result.Errors)
}

func TestMv_SuccessMoveToNewName(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	rootContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "src", InodeID: "src-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(rootContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	srcInode := inode.NewInode("src-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "src-id").Return(srcInode, nil)

	inodeSvc.EXPECT().ReadDir(mock.Anything, "root-id").Return([]inode.DirEntry{}, nil)

	inodeSvc.EXPECT().Link(mock.Anything, "root-id", mock.Anything).Return(nil)

	inodeSvc.EXPECT().Unlink(mock.Anything, "root-id", "src").Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	result, err := svc.Mv(context.Background(), &MvCommand{
		SystemID:    "sys",
		Sources:     []string{"/src"},
		Destination: "/newname",
	})

	assert.NoError(t, err)
	assert.Len(t, result.Moved, 1)
	assert.Equal(t, "/src", result.Moved[0])
}

func TestMv_SourceNotFound(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	rootContent := content.DirContent{Entries: []content.DirEntry{}}
	raw, _ := json.Marshal(rootContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	result, err := svc.Mv(context.Background(), &MvCommand{
		SystemID:    "sys",
		Sources:     []string{"/nonexistent"},
		Destination: "/dest",
	})

	assert.NoError(t, err)
	assert.Empty(t, result.Moved)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "/nonexistent", result.Errors[0].Path)
	assert.True(t, errors.IsNotFound(result.Errors[0].Error))
}

func TestMv_DestinationIsFile(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	rootContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "src", InodeID: "src-id", FileType: uint8(inode.ModeRegular >> 12)},
		{Name: "dest", InodeID: "dest-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(rootContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	srcInode := inode.NewInode("src-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "src-id").Return(srcInode, nil)

	destInode := inode.NewInode("dest-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "dest-id").Return(destInode, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	result, err := svc.Mv(context.Background(), &MvCommand{
		SystemID:    "sys",
		Sources:     []string{"/src"},
		Destination: "/dest",
	})

	assert.NoError(t, err)
	assert.Empty(t, result.Moved)
	assert.Len(t, result.Errors, 1)
	assert.True(t, errors.IsConflict(result.Errors[0].Error))
}

func TestMv_DestinationEntryExists(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	rootContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "src", InodeID: "src-id", FileType: uint8(inode.ModeRegular >> 12)},
		{Name: "dest", InodeID: "dest-id", FileType: uint8(inode.ModeDirectory >> 12)},
	}}
	raw, _ := json.Marshal(rootContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	srcInode := inode.NewInode("src-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "src-id").Return(srcInode, nil)

	destContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "src", InodeID: "existing-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	destRaw, _ := json.Marshal(destContent)
	destInode := inode.NewInode("dest-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, destRaw, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "dest-id").Return(destInode, nil)

	inodeSvc.EXPECT().ReadDir(mock.Anything, "dest-id").Return([]inode.DirEntry{
		{Name: "src", InodeID: "existing-id", FileType: uint8(inode.ModeRegular >> 12)},
	}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	result, err := svc.Mv(context.Background(), &MvCommand{
		SystemID:    "sys",
		Sources:     []string{"/src"},
		Destination: "/dest",
	})

	assert.NoError(t, err)
	assert.Empty(t, result.Moved)
	assert.Len(t, result.Errors, 1)
	assert.True(t, errors.IsConflict(result.Errors[0].Error))
}

func TestMv_MoveDirIntoItself(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	rootContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "src", InodeID: "src-id", FileType: uint8(inode.ModeDirectory >> 12)},
	}}
	raw, _ := json.Marshal(rootContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.On("Find", mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	srcInode := inode.NewInode("src-id", "sys", inode.ModeDirectory|0755, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.On("GetByID", mock.Anything, "src-id").Return(srcInode, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	result, err := svc.Mv(context.Background(), &MvCommand{
		SystemID:    "sys",
		Sources:     []string{"/src"},
		Destination: "/src/subdir",
	})

	assert.NoError(t, err)
	assert.Empty(t, result.Moved)
	assert.Len(t, result.Errors, 1)
	assert.True(t, errors.IsBadRequest(result.Errors[0].Error))
}

func TestMv_SameSourceAndDestination(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil)

	result, err := svc.Mv(context.Background(), &MvCommand{
		SystemID:    "sys",
		Sources:     []string{"/src"},
		Destination: "/src",
	})

	assert.NoError(t, err)
	assert.Empty(t, result.Moved)
	assert.Len(t, result.Errors, 1)
	assert.True(t, errors.IsBadRequest(result.Errors[0].Error))
}

func TestMv_LinkFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	rootContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "src", InodeID: "src-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(rootContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	srcInode := inode.NewInode("src-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "src-id").Return(srcInode, nil)

	inodeSvc.EXPECT().ReadDir(mock.Anything, "root-id").Return([]inode.DirEntry{}, nil)

	inodeSvc.EXPECT().Link(mock.Anything, "root-id", mock.Anything).Return(errors.Internal("link failed"))

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	result, err := svc.Mv(context.Background(), &MvCommand{
		SystemID:    "sys",
		Sources:     []string{"/src"},
		Destination: "/newname",
	})

	assert.NoError(t, err)
	assert.Empty(t, result.Moved)
	assert.Len(t, result.Errors, 1)
}

func TestMv_UnlinkFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	rootContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "src", InodeID: "src-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(rootContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	srcInode := inode.NewInode("src-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "src-id").Return(srcInode, nil)

	inodeSvc.EXPECT().ReadDir(mock.Anything, "root-id").Return([]inode.DirEntry{}, nil)

	inodeSvc.EXPECT().Link(mock.Anything, "root-id", mock.Anything).Return(nil)

	inodeSvc.EXPECT().Unlink(mock.Anything, "root-id", "src").Return(errors.Internal("unlink failed"))

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	result, err := svc.Mv(context.Background(), &MvCommand{
		SystemID:    "sys",
		Sources:     []string{"/src"},
		Destination: "/newname",
	})

	assert.NoError(t, err)
	assert.Empty(t, result.Moved)
	assert.Len(t, result.Errors, 1)
}
