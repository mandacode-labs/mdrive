package vfs

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
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

// --- GetRootDirectory ---

func TestGetRootDirectory_Success(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, nil, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	result, err := svc.GetRootDirectory(context.Background(), "sys")

	assert.NoError(t, err)
	assert.Equal(t, rootInode, result)
}

func TestGetRootDirectory_NotFound(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)

	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	_, err := svc.GetRootDirectory(context.Background(), "sys")

	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestGetRootDirectory_FindFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)

	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return(nil, errors.Internal("find failed"))

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	_, err := svc.GetRootDirectory(context.Background(), "sys")

	assert.Error(t, err)
}

func TestGetRootDirectory_NotDirectory(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	rootInode := inode.NewInode("root-id", "sys", inode.ModeRegular|0644, 0, 0, 0, 1, inode.FlagRoot, now, now, now, nil, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	_, err := svc.GetRootDirectory(context.Background(), "sys")

	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

// --- ResolvePath ---

func TestResolvePath_Root(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, nil, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	result, err := svc.ResolvePath(context.Background(), "sys", "/")

	assert.NoError(t, err)
	assert.Equal(t, rootInode, result)
}

func TestResolvePath_SingleLevel(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	fileInode := inode.NewInode("file-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "file-id").Return(fileInode, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	result, err := svc.ResolvePath(context.Background(), "sys", "/file")

	assert.NoError(t, err)
	assert.Equal(t, fileInode, result)
}

func TestResolvePath_MultiLevel(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	aContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "b", InodeID: "b-id", FileType: uint8(inode.ModeDirectory >> 12)},
	}}
	aRaw, _ := json.Marshal(aContent)
	rootContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "a", InodeID: "a-id", FileType: uint8(inode.ModeDirectory >> 12)},
	}}
	rootRaw, _ := json.Marshal(rootContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, rootRaw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	aInode := inode.NewInode("a-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, aRaw, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "a-id").Return(aInode, nil)

	bContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "c", InodeID: "c-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	bRaw, _ := json.Marshal(bContent)
	bInode := inode.NewInode("b-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, bRaw, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "b-id").Return(bInode, nil)

	cInode := inode.NewInode("c-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "c-id").Return(cInode, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	result, err := svc.ResolvePath(context.Background(), "sys", "/a/b/c")

	assert.NoError(t, err)
	assert.Equal(t, cInode, result)
}

func TestResolvePath_RelativePath(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil)

	_, err := svc.ResolvePath(context.Background(), "sys", "relative/path")

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestResolvePath_EmptyPath(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil)

	_, err := svc.ResolvePath(context.Background(), "sys", "")

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestResolvePath_ComponentNotFound(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	_, err := svc.ResolvePath(context.Background(), "sys", "/nonexistent")

	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestResolvePath_NotADirectory(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	fileInode := inode.NewInode("file-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "file-id").Return(fileInode, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	_, err := svc.ResolvePath(context.Background(), "sys", "/file/subpath")

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestResolvePath_Symlink(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	symContent := content.SymlinkContent{Target: "/"}
	symRaw, _ := json.Marshal(symContent)
	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "link", InodeID: "link-id", FileType: uint8(inode.ModeSymlink >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	linkInode := inode.NewInode("link-id", "sys", inode.ModeSymlink|0777, 0, 0, 0, 1, 0, now, now, now, symRaw, now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "link-id").Return(linkInode, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	result, err := svc.ResolvePath(context.Background(), "sys", "/link")

	assert.NoError(t, err)
	assert.Equal(t, rootInode, result)
}

func TestResolvePath_SymlinkParseError(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeOperationsMock(t)
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "link", InodeID: "link-id", FileType: uint8(inode.ModeSymlink >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, raw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	linkInode := inode.NewInode("link-id", "sys", inode.ModeSymlink|0777, 0, 0, 0, 1, 0, now, now, now, []byte("invalid"), now, now)
	inodeSvc.EXPECT().GetByID(mock.Anything, "link-id").Return(linkInode, nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, nil)

	_, err := svc.ResolvePath(context.Background(), "sys", "/link")

	assert.Error(t, err)
}

// --- Inode.ReadDir ---

func TestInode_ReadDir_Success(t *testing.T) {
	now := time.Now()

	entries := []content.DirEntry{
		{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
	}
	dirContent := content.DirContent{Entries: entries}
	raw, _ := json.Marshal(dirContent)
	dirInode := inode.NewInode("dir-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, raw, now, now)

	result, err := dirInode.ReadDir()

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "file", result[0].Name)
}

func TestInode_ReadDir_NotDirectory(t *testing.T) {
	now := time.Now()

	fileInode := inode.NewInode("file-id", "sys", inode.ModeRegular|0644, 0, 0, 0, 1, 0, now, now, now, nil, now, now)

	_, err := fileInode.ReadDir()

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestInode_ReadDir_NilContent(t *testing.T) {
	now := time.Now()

	dirInode := inode.NewInode("dir-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, nil, now, now)

	result, err := dirInode.ReadDir()

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestInode_ReadDir_UnparsableContent(t *testing.T) {
	now := time.Now()

	dirInode := inode.NewInode("dir-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, []byte("invalid"), now, now)

	_, err := dirInode.ReadDir()

	assert.Error(t, err)
}

// --- Inode.SymlinkTarget ---

func TestInode_SymlinkTarget_Success(t *testing.T) {
	now := time.Now()

	symContent := content.SymlinkContent{Target: "/target"}
	symRaw, _ := json.Marshal(symContent)
	symInode := inode.NewInode("sym-id", "sys", inode.ModeSymlink|0777, 0, 0, 0, 1, 0, now, now, now, symRaw, now, now)

	result, err := symInode.SymlinkTarget()

	assert.NoError(t, err)
	assert.Equal(t, "/target", result)
}

func TestInode_SymlinkTarget_NotSymlink(t *testing.T) {
	now := time.Now()

	fileInode := inode.NewInode("file-id", "sys", inode.ModeRegular|0644, 0, 0, 0, 1, 0, now, now, now, nil, now, now)

	_, err := fileInode.SymlinkTarget()

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestInode_SymlinkTarget_UnparsableContent(t *testing.T) {
	now := time.Now()

	symInode := inode.NewInode("sym-id", "sys", inode.ModeSymlink|0777, 0, 0, 0, 1, 0, now, now, now, []byte("invalid"), now, now)

	_, err := symInode.SymlinkTarget()

	assert.Error(t, err)
}

// --- Inode.IsEmptyDir ---

func TestInode_IsEmptyDir_NilContent(t *testing.T) {
	now := time.Now()

	dirInode := inode.NewInode("dir-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, nil, now, now)

	assert.True(t, dirInode.IsEmptyDir())
}

func TestInode_IsEmptyDir_UnparsableContent(t *testing.T) {
	now := time.Now()

	dirInode := inode.NewInode("dir-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, []byte("invalid"), now, now)

	assert.True(t, dirInode.IsEmptyDir())
}

func TestInode_IsEmptyDir_EmptyDirectory(t *testing.T) {
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{}}
	raw, _ := json.Marshal(dirContent)
	dirInode := inode.NewInode("dir-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, raw, now, now)

	assert.True(t, dirInode.IsEmptyDir())
}

func TestInode_IsEmptyDir_NonEmptyDirectory(t *testing.T) {
	now := time.Now()

	dirContent := content.DirContent{Entries: []content.DirEntry{
		{Name: "file", InodeID: "file-id", FileType: uint8(inode.ModeRegular >> 12)},
	}}
	raw, _ := json.Marshal(dirContent)
	dirInode := inode.NewInode("dir-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, raw, now, now)

	assert.False(t, dirInode.IsEmptyDir())
}

func TestInode_IsEmptyDir_NotDirectory(t *testing.T) {
	now := time.Now()

	fileInode := inode.NewInode("file-id", "sys", inode.ModeRegular|0644, 0, 0, 0, 1, 0, now, now, now, nil, now, now)

	assert.False(t, fileInode.IsEmptyDir())
}
