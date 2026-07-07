package fs

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// CreateObject registers an S3 object as a file-like node.
// Payload lives in S3; the node carries only metadata.
// ActionEdit on parent drive.
func (f *fs) CreateObject(ctx context.Context, driveID, path string, c *ObjectContent) (Stat, error) {
	if c == nil {
		return Stat{}, errorx.New(errorx.KindInvalidArgument, "fs: CreateObject requires content")
	}
	parent, name, err := f.doPathParent(ctx, driveID, path)
	if err != nil {
		return Stat{}, err
	}
	if err := f.requireWrite(ctx, parent.DriveID); err != nil {
		return Stat{}, err
	}
	data, err := c.Marshal()
	if err != nil {
		return Stat{}, errorx.Wrap(err, "fs: marshal object content", errorx.KindInternal)
	}
	node := NewNode(uuid.New(), parent.Node.SuperblockID(), NodeKindObject)
	if err := node.Write(data, c.Size); err != nil {
		return Stat{}, err
	}
	if err := f.vfs.Create(ctx, parent, node, name); err != nil {
		return Stat{}, err
	}
	return NodeToStat(node), nil
}

// ReadObject returns an object-kind node's S3 metadata.
// ActionView.
func (f *fs) ReadObject(ctx context.Context, driveID, path string) (*ObjectContent, error) {
	dentry, err := f.walkForKind(ctx, driveID, path, NodeKindObject)
	if err != nil {
		return nil, err
	}
	if err := f.requireRead(ctx, dentry.DriveID); err != nil {
		return nil, err
	}
	var oc ObjectContent
	if err := json.Unmarshal(dentry.Node.Data(), &oc); err != nil {
		return nil, errorx.Wrap(err, "fs: object content", errorx.KindInternal)
	}
	return &oc, nil
}
