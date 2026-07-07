package superop

import (
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// superOperation is the ent-backed impl of fs.SuperOperation.
type superOperation struct {
	repo Repository
	tm   entx.TxManager
}

// NewSuperblockOperation wires the canonical impl.
func NewSuperblockOperation(repo Repository, tm entx.TxManager) fs.SuperOperation {
	return &superOperation{repo: repo, tm: tm}
}
