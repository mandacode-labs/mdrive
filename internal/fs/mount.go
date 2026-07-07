package fs

import (
	"context"

	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// BindMount attaches another drive at `mountPath`. Mirrors
// sys_mount. ActionEdit on the parent drive, ActionView on
// the source drive (so a non-viewable source cannot be
// mounted).
func (f *fs) BindMount(ctx context.Context, driveID, mountPath, sourceDriveID string) error {
	return f.doBindMount(ctx, driveID, mountPath, sourceDriveID)
}

func (f *fs) doBindMount(ctx context.Context, driveID, mountPath, sourceDriveID string) error {
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
// ActionEdit on the parent drive.
//
// The Linux counterpart also requires CAP_SYS_ADMIN; we
// leave the elevated-privilege check to the OpenFGA model
// and the caller's requireEdit.
func (f *fs) Unmount(ctx context.Context, driveID, mountPath string) error {
	return f.doUnmount(ctx, driveID, mountPath)
}

func (f *fs) doUnmount(ctx context.Context, driveID, mountPath string) error {
	mountParent, mountName, err := f.doPathParent(ctx, driveID, mountPath)
	if err != nil {
		return err
	}
	if err := f.requireEdit(ctx, mountParent.DriveID); err != nil {
		return err
	}
	return f.vfs.Unmount(ctx, mountParent, mountName)
}
