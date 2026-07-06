package nodeop

import (
	"context"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
	"github.com/oklog/ulid/v2"
)

type nodeOperation struct {
	repo NodeRepository
	perm permission.Authorizer
	sb   vfs.SuperOperation
	tm   entx.TxManager
}

// NewNodeOperation wires the canonical impl. nodeop resolves
// drive ids for permission checks via superop.Stat.
func NewNodeOperation(repo NodeRepository, perm permission.Authorizer, sb vfs.SuperOperation, tm entx.TxManager) vfs.NodeOperation {
	return &nodeOperation{
		repo: repo,
		perm: perm,
		sb:   sb,
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

func (n *nodeOperation) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}

// resolveDrive loads the drive id backing a superblock.
func (n *nodeOperation) resolveDrive(ctx context.Context, sb uuid.UUID) (ulid.ULID, error) {
	superblock, err := n.sb.Stat(ctx, sb)
	if err != nil {
		return ulid.ULID{}, errorx.Wrap(err, "nodeop: failed to resolve superblock", errorx.KindInternal)
	}
	return superblock.DriveID(), nil
}

// requirePerm checks drive-level permission. The interface
// accepts an Action but currently hard-codes ActionEdit; the
// sb is resolved to its drive id before checking.
func (n *nodeOperation) requirePerm(ctx context.Context, perm permission.Action, sb uuid.UUID) error {
	driveID, err := n.resolveDrive(ctx, sb)
	if err != nil {
		return err
	}
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
