package driveop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/internal/vfs/permission"
	"github.com/oklog/ulid/v2"
)

type driveOperation struct {
	repo  DriveRepository
	perm  permission.Authorizer
	tm    entx.TxManager
}

func NewDriveOperation(repo DriveRepository, perm permission.Authorizer, tm entx.TxManager) vfs.DriveOperation {
	return &driveOperation{
		repo: repo,
		perm: perm,
		tm:   tm,
	}
}

func (d *driveOperation) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}

func (d *driveOperation) requirePerm(ctx context.Context, perm permission.Action, driveID ulid.ULID) error {
	userID := d.userID(ctx)
	ok, err := d.perm.Check(ctx, userID, permission.ActionEdit, permission.ObjectTypeDrive, driveID.String())
	if err != nil {
		return errorx.Wrap(err, "permission check failed", errorx.KindUnavailable)
	}
	if !ok {
		return errorx.New(errorx.KindPermissionDenied, "permission denied")
	}
	return nil
}