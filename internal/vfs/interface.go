package vfs

import (
	"context"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// PathResolver is the path-only surface of vfs. The upload flow
// uses it to anchor new object nodes and link them into a parent
// directory; it never reads file contents or walks the tree.
type PathResolver interface {
	GetRootNodeID(ctx context.Context, driveID string) (uuid.UUID, error)
	ResolveParentNodeID(ctx context.Context, driveID, path string) (uuid.UUID, string, error)
	ResolveNodeID(ctx context.Context, driveID, path string) (uuid.UUID, error)
}

// Filesystem is the handler-facing vfs surface: all path-driven
// filesystem operations, with permission checks handled by the
// caller before reaching these methods.
type Filesystem interface {
	ResolveForPermission(ctx context.Context, driveID, path string) (PartialResolution, error)
	Mkdir(ctx context.Context, driveID, path string) (*node.Node, error)
	Touch(ctx context.Context, driveID, path string) (*node.Node, error)
	Rm(ctx context.Context, driveID string, paths []string, recursive bool) error
	Mv(ctx context.Context, srcDriveID string, srcPaths []string, dstDriveID, dstPath string) error
	Ls(ctx context.Context, driveID, path string) (node.DirContent, error)
	Cat(ctx context.Context, driveID, path string) ([]byte, error)
	Write(ctx context.Context, driveID, path, content string) error
	WriteLarge(ctx context.Context, driveID, path string, obj node.ObjectContent, size int64) error
	Stat(ctx context.Context, driveID, path string) (*node.Node, error)
	Lstat(ctx context.Context, driveID, path string) (Resolved, error)
	Symlink(ctx context.Context, driveID, target, linkPath string) (*node.Node, error)
	Hardlink(ctx context.Context, driveID, srcPath, linkPath string) (*node.Node, error)
	Mount(ctx context.Context, driveID, mountPath, sourceDriveID string) error
	Unmount(ctx context.Context, driveID, mountPath string) error
}

var (
	_ PathResolver = (*Service)(nil)
	_ Filesystem   = (*Service)(nil)
)
