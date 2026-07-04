package vfs

import (
	"context"

	"github.com/google/uuid"
)

// PathResolver is the path-only surface of vfs. The upload flow
// uses it to anchor new object nodes and link them into a parent
// directory; it never reads file contents or walks the tree.
type PathResolver interface {
	GetRootNodeID(ctx context.Context, driveID string) (uuid.UUID, error)
	ResolveParentNodeID(ctx context.Context, driveID, path string) (uuid.UUID, string, error)
	ResolveNodeID(ctx context.Context, driveID, path string) (uuid.UUID, error)
}

var _ PathResolver = (*Service)(nil)
