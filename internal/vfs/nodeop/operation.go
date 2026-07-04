package nodeop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

type Dentry struct {
	Parent *node.Node
	Name   string
	Node   *node.Node
	Drive  *drive.Drive
}

type NodeOperation interface {
	Mknod(ctx context.Context, dentry *Dentry) error
	Link(ctx context.Context, symlink *Dentry, target *Dentry) error
	Unlink(ctx context.Context, dentry *Dentry) error
	Mkdir(ctx context.Context, dentry *Dentry) error
	Rmdir(ctx context.Context, dentry *Dentry) error
	Rename(ctx context.Context, oldDentry *Dentry, newDentry *Dentry) error
}

type nodeOperation struct {
	super node.SuperOperation
	perm  permission.Authorizer
	tm    entx.TxManager
}

// Rmdir implements [NodeOperation].
func (n *nodeOperation) Rmdir(ctx context.Context, dentry *Dentry) error {
	panic("unimplemented")
}

// Unlink implements [NodeOperation].
func (n *nodeOperation) Unlink(ctx context.Context, dentry *Dentry) error {
	panic("unimplemented")
}

func NewNodeOperation(super node.SuperOperation, tm entx.TxManager) NodeOperation {
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
