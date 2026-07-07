package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/fs/content"
)

// Symlink stores the target's inode id on the symlink node.
func (n *nodeOperation) Symlink(ctx context.Context, symlink *fs.Node, target *fs.Dentry) error {
	if symlink == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: symlink node is nil")
	}
	if symlink.Kind() != fs.NodeKindSymlink {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: node is not a symlink")
	}
	if target == nil || target.Node == nil {
		return errorx.New(errorx.KindInvalidArgument, "nodeop: target is nil")
	}

	c := content.NewSymlinkContent(target.Node.ID())
	data, err := c.Marshal()
	if err != nil {
		return errorx.Wrap(err, "nodeop: marshal symlink content")
	}
	symlink.Write(data, int64(len(data)))

	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		return n.repo.Write(ctx, symlink)
	})
}
