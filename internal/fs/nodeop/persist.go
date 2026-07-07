package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/fs"
)

// Persist writes a single node to storage. Used when only
// the node itself changed (e.g. SetTimes) and there's no
// parent dir entry to update.
func (n *nodeOperation) Persist(ctx context.Context, node *fs.Node) error {
	return n.tm.WithTx(ctx, func(ctx context.Context) error {
		return n.repo.Write(ctx, node)
	})
}
