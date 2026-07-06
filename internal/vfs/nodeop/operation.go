package nodeop

import (
	"context"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

type nodeOperation struct {
	repo NodeRepository
	tm   entx.TxManager
}

// NewNodeOperation wires the canonical impl. Permission is the
// caller's responsibility — nodeop enforces structural invariants
// only.
func NewNodeOperation(repo NodeRepository, tm entx.TxManager) vfs.NodeOperation {
	return &nodeOperation{
		repo: repo,
		tm:   tm,
	}
}

// Get reads an inode by id.
func (n *nodeOperation) Get(ctx context.Context, id uuid.UUID) (*vfs.Node, error) {
	node, err := n.repo.Read(ctx, id)
	if err != nil {
		return nil, errorx.Wrap(err, "nodeop: failed to get node", errorx.KindInternal)
	}
	return node, nil
}
