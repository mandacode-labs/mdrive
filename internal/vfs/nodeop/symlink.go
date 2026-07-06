package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/content"
)

// Symlink implements [NodeOperation].
func (n *nodeOperation) Symlink(ctx context.Context, symlink *vfs.Node, target *vfs.Dentry) error {
	if symlink == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: symlink node is nil")
	}
	if symlink.Kind() != vfs.NodeKindSymlink {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: node is not a symlink")
	}
	if target == nil || target.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: target is nil")
	}

	c := content.NewSymlinkContent(target.Node.ID())
	data, err := c.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: failed to marshal symlink content")
	}
	symlink.Write(data, int64(len(data)))

	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		if err := n.repo.Write(ctx, symlink); err != nil {
			return errorx.Wrap(err, "nodeop: failed to update symlink")
		}
		return nil
	})
}
