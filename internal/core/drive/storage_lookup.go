package drive

import "context"

// StorageLookup is the storage-config-only read surface. The
// upload flow needs bucket/region/etc. but not the drive record
// itself.
type StorageLookup interface {
	GetStorage(ctx context.Context, driveID string) (*Storage, error)
}

var _ StorageLookup = (*Service)(nil)
