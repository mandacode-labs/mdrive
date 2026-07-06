package superop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

type superOperation struct {
	repo Repository
	tm   entx.TxManager
}

func NewSuperblockOperation(repo Repository, tm entx.TxManager) vfs.SuperOperation {
	return &superOperation{
		repo: repo,
		tm:   tm,
	}
}

func (s *superOperation) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}
