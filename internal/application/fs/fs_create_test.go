package fs

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/starfrag-lab/retrowin-go/internal/core/inode"
	"github.com/starfrag-lab/retrowin-go/internal/core/inode/content"
	inodeMocks "github.com/starfrag-lab/retrowin-go/internal/core/inode/mocks"
	userMocks "github.com/starfrag-lab/retrowin-go/internal/core/user/mocks"
	"github.com/starfrag-lab/retrowin-go/internal/errors"
)

func TestCreateFile_MissingSystemID(t *testing.T) {
	svc := NewService(nil, nil, nil, nil)

	_, err := svc.CreateFile(context.Background(), &CreateFileCommand{
		SystemID: "",
	})

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestCreateFile_SuccessWithExplicitUID(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	now := time.Now()

	createdInode := inode.NewInode("new-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(createdInode, nil)

	svc := NewService(inodeSvc, nil, nil, nil)

	result, err := svc.CreateFile(context.Background(), &CreateFileCommand{
		SystemID: "sys",
		UID:      1000,
		GID:      1000,
		Mode:     0644,
	})

	assert.NoError(t, err)
	assert.Equal(t, createdInode, result)
}

func TestCreateFile_SuccessWithResolvedUID(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	userSvc := userMocks.NewUserServiceMock(t)
	now := time.Now()

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000, 1001}, nil)

	createdInode := inode.NewInode("new-id", "sys", inode.ModeRegular|0644, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(createdInode, nil)

	svc := NewService(inodeSvc, nil, userSvc, nil)

	result, err := svc.CreateFile(context.Background(), &CreateFileCommand{
		SystemID: "sys",
		UID:      -1,
		GID:      0,
		Mode:     0644,
	})

	assert.NoError(t, err)
	assert.Equal(t, createdInode, result)
}

func TestCreateFile_DefaultMode(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	now := time.Now()

	createdInode := inode.NewInode("new-id", "sys", inode.ModeRegular|inode.PermOwnerRW|inode.PermGroupRX|inode.PermOtherR, 0, 0, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(createdInode, nil)

	svc := NewService(inodeSvc, nil, nil, nil)

	result, err := svc.CreateFile(context.Background(), &CreateFileCommand{
		SystemID: "sys",
		Mode:     0,
	})

	assert.NoError(t, err)
	assert.Equal(t, createdInode, result)
}

func TestCreateFile_ResolveIDsFailure(t *testing.T) {
	userSvc := userMocks.NewUserServiceMock(t)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(0, nil, errors.Internal("resolve failed"))

	svc := NewService(nil, nil, userSvc, nil)

	_, err := svc.CreateFile(context.Background(), &CreateFileCommand{
		SystemID: "sys",
		UID:      -1,
	})

	assert.Error(t, err)
}

func TestCreateFile_CreateFailure(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)

	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, errors.Internal("create failed"))

	svc := NewService(inodeSvc, nil, nil, nil)

	_, err := svc.CreateFile(context.Background(), &CreateFileCommand{
		SystemID: "sys",
		UID:      0,
		GID:      0,
	})

	assert.Error(t, err)
}

// --- CreateDirectory ---

func TestCreateDirectory_MissingSystemID(t *testing.T) {
	svc := NewService(nil, nil, nil, nil)

	_, err := svc.CreateDirectory(context.Background(), &CreateDirectoryCommand{
		SystemID: "",
	})

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestCreateDirectory_Success(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	now := time.Now()

	createdInode := inode.NewInode("new-id", "sys", inode.ModeDirectory|0755, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(createdInode, nil)
	inodeSvc.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)

	svc := NewService(inodeSvc, nil, nil, nil)

	result, err := svc.CreateDirectory(context.Background(), &CreateDirectoryCommand{
		SystemID: "sys",
		UID:      1000,
		GID:      1000,
		Mode:     0755,
	})

	assert.NoError(t, err)
	assert.Equal(t, createdInode, result)
}

func TestCreateDirectory_DefaultMode(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	now := time.Now()

	createdInode := inode.NewInode("new-id", "sys", inode.ModeDirectory|inode.PermOwnerRWX|inode.PermGroupRX|inode.PermOtherR, 0, 0, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(createdInode, nil)
	inodeSvc.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)

	svc := NewService(inodeSvc, nil, nil, nil)

	result, err := svc.CreateDirectory(context.Background(), &CreateDirectoryCommand{
		SystemID: "sys",
		Mode:     0,
	})

	assert.NoError(t, err)
	assert.Equal(t, createdInode, result)
}

func TestCreateDirectory_DirContentJSON(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	now := time.Now()

	createdInode := inode.NewInode("new-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(createdInode, nil)
	inodeSvc.EXPECT().Update(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, cmd *inode.UpdateCommand) error {
		assert.Equal(t, "new-id", cmd.ID)
		assert.NotNil(t, cmd.Content)
		var dirContent content.DirContent
		err := json.Unmarshal(*cmd.Content, &dirContent)
		assert.NoError(t, err)
		assert.Len(t, dirContent.Entries, 1)
		assert.Equal(t, ".", dirContent.Entries[0].Name)
		assert.Equal(t, "new-id", dirContent.Entries[0].InodeID)
		return nil
	})

	svc := NewService(inodeSvc, nil, nil, nil)

	_, err := svc.CreateDirectory(context.Background(), &CreateDirectoryCommand{
		SystemID: "sys",
		Mode:     0755,
	})

	assert.NoError(t, err)
}

// --- CreateSymlink ---

func TestCreateSymlink_MissingSystemID(t *testing.T) {
	svc := NewService(nil, nil, nil, nil)

	_, err := svc.CreateSymlink(context.Background(), &CreateSymlinkCommand{
		SystemID: "",
		Target:   "/target",
	})

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestCreateSymlink_MissingTarget(t *testing.T) {
	svc := NewService(nil, nil, nil, nil)

	_, err := svc.CreateSymlink(context.Background(), &CreateSymlinkCommand{
		SystemID: "sys",
		Target:   "",
	})

	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestCreateSymlink_Success(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	now := time.Now()

	createdInode := inode.NewInode("new-id", "sys", inode.ModeSymlink|0777, 1000, 1000, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(createdInode, nil)

	svc := NewService(inodeSvc, nil, nil, nil)

	result, err := svc.CreateSymlink(context.Background(), &CreateSymlinkCommand{
		SystemID: "sys",
		Target:   "/target",
		UID:      1000,
		GID:      1000,
		Mode:     0777,
	})

	assert.NoError(t, err)
	assert.Equal(t, createdInode, result)
}

func TestCreateSymlink_SymlinkContentJSON(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	now := time.Now()

	var capturedCmd *inode.CreateCommand
	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, cmd *inode.CreateCommand) (*inode.Inode, error) {
		capturedCmd = cmd
		return inode.NewInode("new-id", "sys", cmd.Mode, cmd.UID, cmd.GID, 0, 1, 0, now, now, now, cmd.Content, now, now), nil
	})

	svc := NewService(inodeSvc, nil, nil, nil)

	_, err := svc.CreateSymlink(context.Background(), &CreateSymlinkCommand{
		SystemID: "sys",
		Target:   "/target",
	})

	assert.NoError(t, err)
	assert.NotNil(t, capturedCmd)
	assert.NotNil(t, capturedCmd.Content)

	var symContent content.SymlinkContent
	err = json.Unmarshal(capturedCmd.Content, &symContent)
	assert.NoError(t, err)
	assert.Equal(t, "/target", symContent.Target)
}

// --- resolveIDs ---

func TestResolveIDs_ExplicitUID(t *testing.T) {
	svc := NewService(nil, nil, nil, nil).(*service)

	uid, gid, err := svc.resolveIDs(context.Background(), "sys", 1000, 1001)

	assert.NoError(t, err)
	assert.Equal(t, 1000, uid)
	assert.Equal(t, 1001, gid)
}

func TestResolveIDs_AutoResolve(t *testing.T) {
	userSvc := userMocks.NewUserServiceMock(t)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000, 1001}, nil)

	svc := NewService(nil, nil, userSvc, nil).(*service)

	uid, gid, err := svc.resolveIDs(context.Background(), "sys", -1, 0)

	assert.NoError(t, err)
	assert.Equal(t, 1000, uid)
	assert.Equal(t, 1000, gid)
}

func TestResolveIDs_AutoResolveWithExplicitGID(t *testing.T) {
	userSvc := userMocks.NewUserServiceMock(t)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(1000, []int{1000, 1001}, nil)

	svc := NewService(nil, nil, userSvc, nil).(*service)

	uid, gid, err := svc.resolveIDs(context.Background(), "sys", -1, 2000)

	assert.NoError(t, err)
	assert.Equal(t, 1000, uid)
	assert.Equal(t, 2000, gid)
}

func TestResolveIDs_AutoResolveFailure(t *testing.T) {
	userSvc := userMocks.NewUserServiceMock(t)

	userSvc.EXPECT().ResolveUIDAndGIDs(mock.Anything, "sys").Return(0, nil, errors.Internal("resolve failed"))

	svc := NewService(nil, nil, userSvc, nil).(*service)

	_, _, err := svc.resolveIDs(context.Background(), "sys", -1, 0)

	assert.Error(t, err)
}
