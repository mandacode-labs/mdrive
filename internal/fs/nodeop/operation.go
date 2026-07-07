package nodeop

import (
	"context"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// nodeOperation is the ent-backed impl of fs.NodeOperation.
// Permission is the caller's responsibility; nodeop enforces
// structural invariants only.
type nodeOperation struct {
	repo NodeRepository
	tm   entx.TxManager
}

// NewNodeOperation wires the canonical impl.
func NewNodeOperation(repo NodeRepository, tm entx.TxManager) fs.NodeOperation {
	return &nodeOperation{repo: repo, tm: tm}
}

// Get reads an inode by id.
func (n *nodeOperation) Get(ctx context.Context, id uuid.UUID) (*fs.Node, error) {
	node, err := n.repo.Read(ctx, id)
	if err != nil {
		return nil, errorx.Wrap(err, "nodeop: get node", errorx.KindInternal)
	}
	return node, nil
}
