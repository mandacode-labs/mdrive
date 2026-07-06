package nodeop

import (
	"context"
	"encoding/json"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
)

// Rmdir implements [NodeOperation]. Removes an empty directory.
// POSIX semantics: refuse if non-empty.
func (n *nodeOperation) Rmdir(ctx context.Context, dentry *vfs.Dentry) error {
	if dentry == nil || dentry.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: nil dentry")
	}
	if dentry.Parent == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: dentry has no parent")
	}
	if dentry.Node.Kind() != vfs.NodeKindDirectory {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: target is not a directory")
	}

	if err := n.requirePerm(ctx, permission.ActionEdit, dentry.Parent.Drive()); err != nil {
		return err
	}

	dirContent := &content.DirContent{}
	if err := json.Unmarshal(dentry.Node.Data(), dirContent); err != nil {
		return errorx.Wrap(err, "nodeop: failed to unmarshal target directory content")
	}
	if len(dirContent.Entries) > 0 {
		return errorx.New(errorx.KindFailedPrecondition, "nodeop: directory not empty")
	}

	parentContent := &content.DirContent{}
	if err := json.Unmarshal(dentry.Parent.Data(), parentContent); err != nil {
		return errorx.Wrap(err, "nodeop: failed to unmarshal parent directory content")
	}
	if err := parentContent.RemoveEntry(dentry.Name); err != nil {
		return errorx.Wrap(err, "nodeop: failed to remove entry from parent directory")
	}
	newParentData, err := parentContent.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: failed to marshal parent directory content")
	}
	dentry.Parent.Write(newParentData, int64(len(newParentData)))

	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, dentry.Parent); err != nil {
			return errorx.Wrap(err, "nodeop: failed to update parent directory")
		}
		if err := n.repo.Destroy(ctx, dentry.Node.ID()); err != nil {
			return errorx.Wrap(err, "nodeop: failed to destroy directory inode")
		}
		return nil
	})
}
