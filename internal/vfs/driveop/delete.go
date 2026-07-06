package driveop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// DeleteDrive implements [vfs.DriveOperation].
func (d *driveOperation) DeleteDrive(ctx context.Context, drive *vfs.Drive) error {
	panic("unimplemented")
}
