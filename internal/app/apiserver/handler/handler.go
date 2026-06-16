package handler

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	api "github.com/mandacode-labs/mdrive/pkg/api"
)

// FS is the consumer-declared VFS interface.
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
	CreateDrive(ctx context.Context, actorID string, name, description string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error)
	GetDrive(ctx context.Context, id string) (*drive.Drive, error)
	GetDriveStorage(ctx context.Context, driveID string) (*drive.Storage, error)
	UpdateDrive(ctx context.Context, id string, name, description *string) (*drive.Drive, error)
	DeleteDrive(ctx context.Context, id string) error
	ListUserDrives(ctx context.Context, actorID string) ([]*drive.Drive, error)
}

// Handler implements the ogen Handler interface.
type Handler struct {
	vfs     FS
	getUser func(context.Context) (string, bool)
}

func New(fs FS, getUser func(context.Context) (string, bool)) *Handler {
	return &Handler{vfs: fs, getUser: getUser}
}

func (h *Handler) userID(ctx context.Context) string {
	id, _ := h.getUser(ctx)
	return id
}

// Compile-time check.
var _ api.Handler = (*Handler)(nil)
var _ = fmt.Sprintf
