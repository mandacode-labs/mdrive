package fs

import (
	"context"
	"time"

	"github.com/mandacode-labs/mdrive/internal/perm"
)

// Service is the syscall surface for the fs subsystem.
// Methods walk the path, check permission, dispatch into
// the vfs layer, and decode payloads into typed content.
type Service interface {
	// === File ops (NodeKindFile) — inline payload up to 4KB ===
	CreateFile(ctx context.Context, driveID, path string, c *FileContent) (Stat, error)
	ReadFile(ctx context.Context, driveID, path string) (*FileContent, error)
	WriteFile(ctx context.Context, driveID, path string, c *FileContent) (Stat, error)
	Truncate(ctx context.Context, driveID, path string, size int64) error

	// === Object ops (NodeKindObject) — S3-backed ===
	CreateObject(ctx context.Context, driveID, path string, c *ObjectContent) (Stat, error)
	ReadObject(ctx context.Context, driveID, path string) (*ObjectContent, error)

	// === Storage ops (S3 presign) ===
	// Download returns a presigned GET URL. ActionRead.
	Download(ctx context.Context, driveID, path string, expiry time.Duration) (string, error)
	// Upload returns a presigned PUT URL + Key. ActionWrite + ActionUpload.
	Upload(ctx context.Context, driveID, path, contentType string, expiry time.Duration) (UploadInfo, error)
	// Complete verifies the S3 object (size from backend) and
	// creates the object-kind node. ActionUpload.
	Complete(ctx context.Context, driveID, path, key string) (Stat, error)

	// === Directory ops (NodeKindDirectory) ===
	Mkdir(ctx context.Context, driveID, path string) (Stat, error)
	Getdents(ctx context.Context, driveID, path string) (*DirContent, error)

	// === Symlink ops ===
	SymlinkAt(ctx context.Context, driveID, target, linkPath string) (Stat, error)
	ReadlinkAt(ctx context.Context, driveID, path string) (*SymlinkContent, error)

	// === Link ops ===
	LinkAt(ctx context.Context, driveID, srcPath, linkPath string) (Stat, error)
	Unlink(ctx context.Context, driveID, path string) error
	Rmdir(ctx context.Context, driveID, path string) error

	// === Tree ops ===
	RenameAt(ctx context.Context, driveID, srcPath, dstDriveID, dstPath string) error
	Remove(ctx context.Context, driveID string, paths []string, opts RemoveOpts) error

	// === Metadata ===
	Stat(ctx context.Context, driveID, path string, follow bool) (Stat, error)
	SetTimes(ctx context.Context, driveID, path string, atime, mtime time.Time) error

	// === Mount ops ===
	BindMount(ctx context.Context, driveID, mountPath, sourceDriveID string) error
	Unmount(ctx context.Context, driveID, mountPath string) error
}

// RemoveOpts controls Remove behavior.
type RemoveOpts struct {
	Recursive bool
}

// fs is the concrete Service.
type fs struct {
	vfs  VFS
	perm perm.Service
}

// Config groups the dependencies of New. Storage handling
// lives inside the VFS layer (vfs.Config.StorageOp +
// vfs.Config.DefaultS3).
type Config struct {
	VFS  VFS
	Perm perm.Service
}

// New wires a Service.
func New(cfg Config) Service {
	return &fs{
		vfs:  cfg.VFS,
		perm: cfg.Perm,
	}
}