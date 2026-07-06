package vfs

import (
	"context"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// SuperOperation is the minimal contract vfs needs to access
type SuperOperation interface {
	GetRootNodeID(ctx context.Context, driveID ulid.ULID) (uuid.UUID, error)
}
