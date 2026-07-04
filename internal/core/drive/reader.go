package drive

import "context"

// Reader is the read-only surface of a drive service. vfs uses
// it to resolve drive root IDs and storage config; permission
// checks are the caller's responsibility.
type Reader interface {
	GetByID(ctx context.Context, id string) (*Drive, error)
	GetByPublicID(ctx context.Context, publicID string) (*Drive, error)
	GetStorage(ctx context.Context, driveID string) (*Storage, error)
}

var _ Reader = (*Service)(nil)
