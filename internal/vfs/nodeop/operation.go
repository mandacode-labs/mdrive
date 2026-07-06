package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
	"github.com/oklog/ulid/v2"
)

type nodeOperation struct {
	bs   BlockStorage
	perm permission.Authorizer
	tm   entx.TxManager
}

func NewNodeOperation(bs BlockStorage, perm permission.Authorizer, tm entx.TxManager) vfs.NodeOperation {
	return &nodeOperation{
		bs:   bs,
		perm: perm,
		tm:   tm,
	}
}

func (n *nodeOperation) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}

func (n *nodeOperation) requirePerm(ctx context.Context, perm permission.Action, driveID ulid.ULID) error {
	userID := n.userID(ctx)
	ok, err := n.perm.Check(ctx, userID, permission.ActionEdit, permission.ObjectTypeDrive, driveID.String())
	if err != nil {
		return errorx.Wrap(err, "permission check failed", errorx.KindUnavailable)
	}
	if !ok {
		return errorx.New(errorx.KindPermissionDenied, "permission denied")
	}
	return nil
}
