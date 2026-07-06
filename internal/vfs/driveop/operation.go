package driveop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/crypto"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
	"github.com/oklog/ulid/v2"
)

// RootNodeCreator creates the root directory inode for a freshly
// created drive. Implemented by nodeop (or a stub during testing)
// to avoid an upward import from vfs/driveop into vfs/nodeop.
type RootNodeCreator interface {
	CreateRootDirectory(ctx context.Context) (*vfs.Node, error)
}

// OwnerChecker verifies that an actor (user ID) exists before a
// drive can be created against them. user.Repository satisfies
// this via its Exist method.
type OwnerChecker interface {
	Exist(ctx context.Context, id string) (bool, error)
}

type driveOperation struct {
	repo    DriveRepository
	storage StorageRepository
	perm    permission.Authorizer
	tm      entx.TxManager
	cipher  crypto.Cipher
	root    RootNodeCreator
	owner   OwnerChecker
}

func NewDriveOperation(
	repo DriveRepository,
	storage StorageRepository,
	perm permission.Authorizer,
	tm entx.TxManager,
	cipher crypto.Cipher,
	root RootNodeCreator,
	owner OwnerChecker,
) vfs.DriveOperation {
	if cipher == nil {
		cipher = crypto.NoOp{}
	}
	return &driveOperation{
		repo:    repo,
		storage: storage,
		perm:    perm,
		tm:      tm,
		cipher:  cipher,
		root:    root,
		owner:   owner,
	}
}

var _ vfs.DriveOperation = (*driveOperation)(nil)

func (d *driveOperation) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}

func (d *driveOperation) requirePerm(ctx context.Context, action permission.Action, driveID ulid.ULID) error {
	userID := d.userID(ctx)
	ok, err := d.perm.Check(ctx, userID, action, permission.ObjectTypeDrive, driveID.String())
	if err != nil {
		return errorx.Wrap(err, "permission check failed", errorx.KindUnavailable)
	}
	if !ok {
		return errorx.New(errorx.KindPermissionDenied, "permission denied")
	}
	return nil
}

// driveIDFromString parses a string drive id into ulid.ULID,
// returning KindInvalidArgument on failure.
func driveIDFromString(id string) (ulid.ULID, error) {
	driveID, err := ulid.Parse(id)
	if err != nil {
		return ulid.ULID{}, errorx.Wrap(err, "driveop: invalid drive id", errorx.KindInvalidArgument)
	}
	return driveID, nil
}
