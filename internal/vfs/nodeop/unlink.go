package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// Unlink implements [NodeOperation].
func (n *nodeOperation) Unlink(ctx context.Context, dentry *vfs.Dentry) error {
	panic("unimplemented")
}
