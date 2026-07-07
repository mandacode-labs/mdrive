package fs

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// CreateFile inserts a file-kind node with the given content.
// Mirrors creat(2) + write(2). ActionEdit on parent drive.
func (f *fs) CreateFile(ctx context.Context, driveID, path string, c *FileContent) (Stat, error) {
	if c == nil {
		return Stat{}, errorx.New(errorx.KindInvalidArgument, "fs: CreateFile requires content")
	}
	parent, name, err := f.doPathParent(ctx, driveID, path)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireEdit(ctx, parent.DriveID); err != nil {
		return Stat{}, err
	}
	data, err := c.Marshal()
	if err != nil {
		return Stat{}, errorx.Wrap(err, "fs: marshal file content", errorx.KindInternal)
	}
	node := NewNode(uuid.New(), parent.Node.SuperblockID(), NodeKindFile)
	if err := node.Write(data, int64(len(data))); err != nil {
		return Stat{}, err
	}
	if err := f.vfs.Create(ctx, parent, node, name); err != nil {
		return Stat{}, err
	}
	return NodeToStat(node), nil
}

// ReadFile returns the inline payload of a file-kind node
// decoded as *FileContent. ActionView on the
// resolved drive.
func (f *fs) ReadFile(ctx context.Context, driveID, path string) (*FileContent, error) {
	dentry, err := f.walkForKind(ctx, driveID, path, NodeKindFile)
	if err != nil {
		return nil, err
	}
	if err := f.requireView(ctx, dentry.DriveID); err != nil {
		return nil, err
	}
	var fc FileContent
	if err := json.Unmarshal(dentry.Node.Data(), &fc); err != nil {
		return nil, errorx.Wrap(err, "fs: file content", errorx.KindInternal)
	}
	return &fc, nil
}

// WriteFile replaces the inline payload of an existing
// file-kind node. ActionEdit on the resolved drive.
func (f *fs) WriteFile(ctx context.Context, driveID, path string, c *FileContent) (Stat, error) {
	if c == nil {
		return Stat{}, errorx.New(errorx.KindInvalidArgument, "fs: WriteFile requires content")
	}
	dentry, err := f.walkForKind(ctx, driveID, path, NodeKindFile)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireEdit(ctx, dentry.DriveID); err != nil {
		return Stat{}, err
	}
	data, err := c.Marshal()
	if err != nil {
		return Stat{}, errorx.Wrap(err, "fs: marshal file content", errorx.KindInternal)
	}
	if err := f.vfs.Write(ctx, dentry, data); err != nil {
		return Stat{}, err
	}
	return NodeToStat(dentry.Node), nil
}

// Truncate sets the size of a file-kind node. Currently a
// no-op on the byte stream — inline data is wholly
// replaced by WriteFile. ActionEdit on the resolved drive.
func (f *fs) Truncate(ctx context.Context, driveID, path string, size int64) error {
	dentry, err := f.walkForKind(ctx, driveID, path, NodeKindFile)
	if err != nil {
		return err
	}
	if err := f.requireEdit(ctx, dentry.DriveID); err != nil {
		return err
	}
	if size < 0 {
		return errorx.New(errorx.KindInvalidArgument, "fs: Truncate size is negative")
	}
	if size > MaxDataSize {
		return errorx.New(errorx.KindInvalidArgument, "fs: Truncate exceeds MaxDataSize")
	}
	return f.vfs.Write(ctx, dentry, make([]byte, size))
}