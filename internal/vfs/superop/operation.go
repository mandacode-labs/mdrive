package superop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// superOperation is the ent-backed impl of vfs.SuperOperation.
// Create / Stat / Purge are stubbed; see superop/sbrepo.go for
// the actual CRUD that the ent repository exposes.
type superOperation struct {
	repo Repository
	tm   entx.TxManager
}

// NewSuperblockOperation wires the canonical impl.
func NewSuperblockOperation(repo Repository, tm entx.TxManager) vfs.SuperOperation {
	return &superOperation{
		repo: repo,
		tm:   tm,
	}
}

func (s *superOperation) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}
