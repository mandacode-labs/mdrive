package fs

import (
	"context"

	"github.com/mandacode-labs/retrowin-go/ent"
	"github.com/mandacode-labs/retrowin-go/internal/core/dentry"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/core/object"
	"github.com/mandacode-labs/retrowin-go/internal/core/user"
)

// FsService defines the interface for filesystem operations.
type FsService interface {
	CreateFile(ctx context.Context, cmd *CreateFileCommand) (*inode.Inode, error)
	CreateDirectory(ctx context.Context, cmd *CreateDirectoryCommand) (*inode.Inode, error)
	CreateSymlink(ctx context.Context, cmd *CreateSymlinkCommand) (*inode.Inode, error)
	Get(ctx context.Context, id string) (*inode.Inode, error)
	UpdateContent(ctx context.Context, cmd *UpdateContentCommand) (*inode.Inode, error)
	UpdateMode(ctx context.Context, cmd *UpdateModeCommand) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter *ListFilter) ([]*inode.Inode, error)
	Copy(ctx context.Context, id string, systemID string) (*inode.Inode, error)

	// GetRootDirectory returns the root directory for a system.
	GetRootDirectory(ctx context.Context, systemID string) (*inode.Inode, error)
	// ResolvePath resolves a Unix-style path to an inode.
	// Path must be absolute (start with /).
	ResolvePath(ctx context.Context, systemID string, path string) (*inode.Inode, error)

	// Mkdir creates a directory at the given path.
	Mkdir(ctx context.Context, systemID string, path string, mode int) (*inode.Inode, error)
	// Ln creates a symbolic link at linkPath pointing to target.
	Ln(ctx context.Context, systemID string, linkPath string, target string) (*inode.Inode, error)
	// UnlinkPath removes a path (file or directory).
	UnlinkPath(ctx context.Context, systemID string, path string) error
	// ChmodPath changes permissions of a path.
	ChmodPath(ctx context.Context, systemID string, path string, mode int) (*inode.Inode, error)

	// Rm removes multiple paths. Like Unix rm, calls unlinkat + inode delete per path.
	// If Recursive is true, directories are deleted recursively.
	Rm(ctx context.Context, cmd *RmCommand) (*RmResult, error)
	// Mv moves multiple paths to a destination. Like Unix mv, uses renameat per source.
	Mv(ctx context.Context, cmd *MvCommand) (*MvResult, error)
	// Rename renames a single entry within the same directory. Uses renameat.
	Rename(ctx context.Context, cmd *RenameCommand) (*inode.Inode, error)

	// AtomicUpload completes an upload and links the file to the filesystem atomically.
	// Uses a transaction to ensure object, inode, and dentry updates are all-or-nothing.
	AtomicUpload(ctx context.Context, cmd *AtomicUploadCommand) (*inode.Inode, error)
	// DeleteRecursive recursively deletes a directory and all its contents.
	// S3 objects are deleted before their DB records to prevent orphan storage costs.
	DeleteRecursive(ctx context.Context, systemID string, path string) error
}

// CreateFileCommand for creating a regular file.
type CreateFileCommand struct {
	SystemID string
	UID      int
	GID      int
	Mode     int
	Size     int64
	Flags    int
	Content  []byte
}

// CreateDirectoryCommand for creating a directory.
type CreateDirectoryCommand struct {
	SystemID string
	UID      int
	GID      int
	Mode     int
	Flags    int
}

// CreateSymlinkCommand for creating a symbolic link.
type CreateSymlinkCommand struct {
	SystemID string
	UID      int
	GID      int
	Mode     int
	Flags    int
	Target   string
}

// UpdateContentCommand for updating file content.
type UpdateContentCommand struct {
	ID      string
	Content []byte
}

// UpdateModeCommand for updating file mode (permissions).
type UpdateModeCommand struct {
	ID   string
	Mode int
}

// ListFilter for listing inodes.
type ListFilter struct {
	SystemID *string
	UID      *int
}

// RmCommand for bulk removal of paths.
type RmCommand struct {
	SystemID  string
	Paths     []string
	Recursive bool
}

// RmResult contains the results of a bulk rm operation.
type RmResult struct {
	Deleted []string // successfully deleted paths
	Errors  []RmError
}

// RmError represents a per-path error during rm.
type RmError struct {
	Path  string
	Error error
}

// MvCommand for bulk move of paths to a destination.
type MvCommand struct {
	SystemID    string
	Sources     []string
	Destination string
}

// MvResult contains the results of a bulk mv operation.
type MvResult struct {
	Moved  []string // successfully moved paths
	Errors []MvError
}

// MvError represents a per-path error during mv.
type MvError struct {
	Path  string
	Error error
}

// RenameCommand for renaming an entry within the same directory.
type RenameCommand struct {
	SystemID string
	Path     string
	NewName  string
}

// AtomicUploadCommand atomically completes an upload and links it to the filesystem.
type AtomicUploadCommand struct {
	ObjectID string
	SystemID string
	DirPath  string
	FileName string
	Mode     int
	Flags    int
}

type service struct {
	entClient *ent.Client
	inodeSvc  inode.InodeService
	objectSvc object.ObjectService
	storage   object.Storage
	userSvc   user.UserService
	dentrySvc dentry.DentryService
	dirLock   *dentry.Locker
}

// NewService creates a new filesystem service.
func NewService(entClient *ent.Client, inodeSvc inode.InodeService, objectSvc object.ObjectService, storage object.Storage, userSvc user.UserService, dentrySvc dentry.DentryService, dirLock *dentry.Locker) FsService {
	return &service{
		entClient: entClient,
		inodeSvc:  inodeSvc,
		objectSvc: objectSvc,
		storage:   storage,
		userSvc:   userSvc,
		dentrySvc: dentrySvc,
		dirLock:   dirLock,
	}
}
