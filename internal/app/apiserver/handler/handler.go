// Package handler implements the HTTP API handlers for the apiserver.
package handler

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// FS is the consumer-declared VFS interface for path-based filesystem operations.
// Declared here (the consumer) to follow Go convention, not in the vfs package.
// The vfs.Service implicitly satisfies this interface.
type FS interface {
	Mkdir(ctx context.Context, userID, driveID, path string) (*node.Node, error)
	Touch(ctx context.Context, userID, driveID, path string) (*node.Node, error)
	Rm(ctx context.Context, userID, driveID string, paths []string, recursive bool) error
	Mv(ctx context.Context, userID, srcDriveID string, srcPaths []string, dstDriveID, dstPath string) error
	Ls(ctx context.Context, userID, driveID, path string) (node.DirContent, error)
	Cat(ctx context.Context, userID, driveID, path string) ([]byte, error)
	Write(ctx context.Context, userID, driveID, path, content string) error
	WriteLarge(ctx context.Context, userID, driveID, path string, obj node.ObjectContent, size int64) error
	Stat(ctx context.Context, userID, driveID, path string) (*node.Node, error)
	Symlink(ctx context.Context, userID, driveID, target, linkPath string) (*node.Node, error)
}
