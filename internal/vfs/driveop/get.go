package driveop

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// GetDrive implements [vfs.DriveOperation].
func (d *driveOperation) GetDrive(ctx context.Context, driveID string) (*vfs.Drive, error) {
	panic("unimplemented")
}
