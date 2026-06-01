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
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func TestMkdir_RootPath(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.Mkdir(context.Background(), "sys", "/", 0755)
	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestMkdir_DotNameRejected(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.Mkdir(context.Background(), "sys", "/home/.", 0755)
	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestMkdir_DotDotNameRejected(t *testing.T) {
	svc := NewService(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.Mkdir(context.Background(), "sys", "/home/..", 0755)
	assert.Error(t, err)
	assert.True(t, errors.IsBadRequest(err))
}

func TestMkdir_LinkFailureRollback(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
	now := time.Now()

	// ResolvePath mocks
	rootContent := content.DirContent{Entries: []content.DirEntry{}}
	rootRaw, _ := json.Marshal(rootContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, rootRaw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	// CreateDirectory mocks
	createdInode := inode.NewInode("new-dir-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(createdInode, nil)
	inodeSvc.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)

	// Link fails
	dentrySvc.EXPECT().Link(mock.Anything, "root-id", mock.Anything).Return(errors.Conflict("entry already exists"))

	// Rollback: delete the orphaned inode
	inodeSvc.EXPECT().Delete(mock.Anything, "new-dir-id").Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, dentrySvc, nil)

	_, err := svc.Mkdir(context.Background(), "sys", "/testdir", 0755)
	assert.Error(t, err)
	assert.True(t, errors.IsConflict(err))
}

func TestMkdir_DotDotLinkFailureRollback(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
	now := time.Now()

	// ResolvePath mocks
	rootContent := content.DirContent{Entries: []content.DirEntry{}}
	rootRaw, _ := json.Marshal(rootContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, rootRaw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	// CreateDirectory mocks
	createdInode := inode.NewInode("new-dir-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(createdInode, nil)
	inodeSvc.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)

	// First Link succeeds (link into parent)
	dentrySvc.EXPECT().Link(mock.Anything, "root-id", mock.Anything).Return(nil)

	// Second Link fails (.. entry)
	dentrySvc.EXPECT().Link(mock.Anything, "new-dir-id", mock.MatchedBy(func(e dentry.DirEntry) bool {
		return e.Name == ".."
	})).Return(errors.Internal("link failed"))

	// Rollback: unlink from parent and delete inode
	dentrySvc.EXPECT().Unlink(mock.Anything, "root-id", "testdir").Return(nil)
	inodeSvc.EXPECT().Delete(mock.Anything, "new-dir-id").Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, dentrySvc, nil)

	_, err := svc.Mkdir(context.Background(), "sys", "/testdir", 0755)
	assert.Error(t, err)
}

func TestMkdir_Success(t *testing.T) {
	inodeSvc := inodeMocks.NewInodeServiceMock(t)
	dentrySvc := dentryMocks.NewDentryServiceMock(t)
	now := time.Now()

	// ResolvePath mocks
	rootContent := content.DirContent{Entries: []content.DirEntry{}}
	rootRaw, _ := json.Marshal(rootContent)
	rootInode := inode.NewInode("root-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, inode.FlagRoot, now, now, now, rootRaw, now, now)
	inodeSvc.EXPECT().Find(mock.Anything, mock.Anything).Return([]*inode.Inode{rootInode}, nil)

	// CreateDirectory mocks
	createdInode := inode.NewInode("new-dir-id", "sys", inode.ModeDirectory|0755, 0, 0, 0, 1, 0, now, now, now, nil, now, now)
	inodeSvc.EXPECT().Create(mock.Anything, mock.Anything).Return(createdInode, nil)
	inodeSvc.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)

	// Link into parent
	dentrySvc.EXPECT().Link(mock.Anything, "root-id", mock.MatchedBy(func(e dentry.DirEntry) bool {
		return e.Name == "testdir" && e.InodeID == "new-dir-id"
	})).Return(nil)

	// Add .. entry
	dentrySvc.EXPECT().Link(mock.Anything, "new-dir-id", mock.MatchedBy(func(e dentry.DirEntry) bool {
		return e.Name == ".." && e.InodeID == "root-id"
	})).Return(nil)

	svc := NewService(nil, inodeSvc, nil, nil, nil, dentrySvc, nil)

	result, err := svc.Mkdir(context.Background(), "sys", "/testdir", 0755)
	assert.NoError(t, err)
	assert.Equal(t, createdInode, result)
}
