package superop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// superOperation is the ent-backed impl of fs.SuperOperation.
// Create / Stat / Purge are stubbed; the actual CRUD lives
// on Repository.
type superOperation struct {
	repo Repository
	tm   entx.TxManager
}

// NewSuperblockOperation wires the canonical impl.
func NewSuperblockOperation(repo Repository, tm entx.TxManager) fs.SuperOperation {
	return &superOperation{repo: repo, tm: tm}
}

func (s *superOperation) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}
