package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

type nodeOperation struct {
	super node.SuperOperation
	perm  permission.Authorizer
	tm    entx.TxManager
}

func NewNodeOperation(super node.SuperOperation, tm entx.TxManager) vfs.NodeOperation {
	return &nodeOperation{
		super: super,
		tm:    tm,
	}
}

func (n *nodeOperation) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}

func (n *nodeOperation) requirePerm(ctx context.Context, perm permission.Action, driveID string) error {
	userID := n.userID(ctx)
	ok, err := n.perm.Check(ctx, userID, permission.ActionEdit, permission.ObjectTypeDrive, driveID)
	if err != nil {
		return errorx.Wrap(err, "permission check failed", errorx.KindUnavailable)
	}
	if !ok {
		return errorx.New(errorx.KindPermissionDenied, "permission denied")
	}
	return nil
}
