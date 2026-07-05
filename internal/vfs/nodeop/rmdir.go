package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// Rmdir implements [NodeOperation].
func (n *nodeOperation) Rmdir(ctx context.Context, dentry *vfs.Dentry) error {
	panic("unimplemented")
}
