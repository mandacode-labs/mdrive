package fs

import (
	"context"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/perm"
)

// userID extracts the caller's user id from the request ctx.
func (f *fs) userID(ctx context.Context) string {
	return auth.UserIDFromContext(ctx)
}

// requireRead gates path resolution + read syscalls.
func (f *fs) requireRead(ctx context.Context, driveID ulid.ULID) error {
	return f.perm.Check(ctx, f.userID(ctx), perm.ActionRead, perm.ObjectTypeDrive, driveID.String())
}

// requireWrite gates mutating syscalls on existing nodes.
func (f *fs) requireWrite(ctx context.Context, driveID ulid.ULID) error {
	return f.perm.Check(ctx, f.userID(ctx), perm.ActionWrite, perm.ObjectTypeDrive, driveID.String())
}

// requireDelete gates unlink/rmdir/remove.
func (f *fs) requireDelete(ctx context.Context, driveID ulid.ULID) error {
	return f.perm.Check(ctx, f.userID(ctx), perm.ActionDelete, perm.ObjectTypeDrive, driveID.String())
}

// requireUpload gates presigned PUT issuance and completion.
func (f *fs) requireUpload(ctx context.Context, driveID ulid.ULID) error {
	return f.perm.Check(ctx, f.userID(ctx), perm.ActionUpload, perm.ObjectTypeDrive, driveID.String())
}

// requireManageStorage gates bind-mount/umount.
func (f *fs) requireManageStorage(ctx context.Context, driveID ulid.ULID) error {
	return f.perm.Check(ctx, f.userID(ctx), perm.ActionManageStorage, perm.ObjectTypeDrive, driveID.String())
}