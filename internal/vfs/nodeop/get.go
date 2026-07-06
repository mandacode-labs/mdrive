package nodeop

import (
	"context"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// Get reads an inode by id.
func (n *nodeOperation) Get(ctx context.Context, id uuid.UUID) (*vfs.Node, error) {
	node, err := n.repo.Read(ctx, id)
	if err != nil {
		return nil, errorx.Wrap(err, "nodeop: failed to get node", errorx.KindInternal)
	}
	return node, nil
}

