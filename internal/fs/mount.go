package fs

import (
	"context"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// BindMount attaches another drive at mountPath. Mirrors
// sys_mount. ActionEdit on parent, ActionView on source
// (so a non-viewable source cannot be mounted).
func (f *fs) BindMount(ctx context.Context, driveID, mountPath, sourceDriveID string) error {
	mountParent, mountName, err := f.doPathParent(ctx, driveID, mountPath)
	if err != nil {
		return err
	}
	if err := f.requireEdit(ctx, mountParent.DriveID); err != nil {
		return err
	}
	srcULID, err := ulid.Parse(sourceDriveID)
	if err != nil {
		return errorx.Wrap(err, "fs: invalid source drive id", errorx.KindInvalidArgument)
	}
	if err := f.requireView(ctx, srcULID); err != nil {
		return err
	}
	return f.vfs.Mount(ctx, mountParent, mountName, srcULID)
}

// Unmount detaches a bind mount. Mirrors sys_umount2.
// ActionEdit.
//
// The Linux counterpart requires CAP_SYS_ADMIN; we leave
// the elevated-privilege check to OpenFGA.
func (f *fs) Unmount(ctx context.Context, driveID, mountPath string) error {
	mountParent, mountName, err := f.doPathParent(ctx, driveID, mountPath)
	if err != nil {
		return err
	}
	if err := f.requireEdit(ctx, mountParent.DriveID); err != nil {
		return err
	}
	return f.vfs.Unmount(ctx, mountParent, mountName)
}
