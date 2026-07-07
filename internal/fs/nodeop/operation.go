package nodeop

import (
	"github.com/mandacode-labs/mdrive/internal/entx"
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
