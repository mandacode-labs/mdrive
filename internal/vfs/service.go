package vfs

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// GarbageRef points at an external (S3) object that needs cleanup
// when an inode is removed.
type GarbageRef struct {
	Bucket string
	Key    string
}

// GarbageRecorder records tombstones for deleted S3 objects. A
// nil GarbageRecorder means vfs skips tombstoning. The interface
// stays in vfs (the only consumer); the concrete *gc.Recorder
// satisfies it structurally.
type GarbageRecorder interface {
	RecordGarbage(ctx context.Context, refs []GarbageRef) error
}

// Service is the public contract of the virtual filesystem. It
// composes a node operation and a drive service, walks POSIX-style
// paths, follows mounts, and records S3 tombstones for removed
// object nodes. Permission checks are the caller's responsibility.
//
// Callers depend on this single interface; the unexported service
// struct is the only implementation.
type Service interface {
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
	Resolve(ctx context.Context, driveID, path string) (Resolved, error)
	GetRootNodeID(ctx context.Context, driveID string) (uuid.UUID, error)
	ResolveParentNodeID(ctx context.Context, driveID, path string) (uuid.UUID, string, error)
	ResolveNodeID(ctx context.Context, driveID, path string) (uuid.UUID, error)
}

// service is the only implementation of Service.
type service struct {
	NodeClient      node.NodeOperation
	DriveClient     drive.Service
	GarbageRecorder GarbageRecorder
	tm              entx.TxManager
}

// Config groups the dependencies of NewService.
type Config struct {
	NodeClient      node.NodeOperation
	DriveClient     drive.Service
	GarbageRecorder GarbageRecorder
	TxManager       entx.TxManager
}

// NewService wires a service.
func NewService(cfg Config) Service {
	return &service{
		NodeClient:      cfg.NodeClient,
		DriveClient:     cfg.DriveClient,
		GarbageRecorder: cfg.GarbageRecorder,
		tm:              cfg.TxManager,
	}
}

var _ Service = (*service)(nil)

// Resolved is the result of a path lookup that follows mounts and
// symlinks: the drive the final node lives in (which may differ
// from the requested driveID if mounts were crossed) and the node
// itself.
type Resolved struct {
	DriveID string
	Node    *node.Node
}

const maxMountHops = 32

func (s *service) Resolve(ctx context.Context, driveID, path string) (Resolved, error) {
	drive, n, err := s.resolveWithMounts(ctx, driveID, path, true)
	if err != nil {
		return Resolved{}, err
	}
	if n == nil {
		return Resolved{}, errorx.New(errorx.KindNotFound, "vfs: not found")
	}
	return Resolved{DriveID: drive, Node: n}, nil
}

// PartialResolution is the partial resolution returned by
// ResolveForPermission: the drive the path lands in (after stopping
// at the first mount) and the remaining path within that drive.
type PartialResolution struct {
	DriveID string
	Path    string
}

func (s *service) ResolveForPermission(ctx context.Context, driveID, path string) (PartialResolution, error) {
	rootID, err := s.GetRootNodeID(ctx, driveID)
	if err != nil {
		return PartialResolution{}, err
	}
	r := newResolver(s.NodeClient)
	out, err := r.resolvePath(ctx, rootID, path, true)
	if err != nil {
		return PartialResolution{}, err
	}
	if out.Node == nil {
		return PartialResolution{}, errorx.New(errorx.KindNotFound, "vfs: not found")
	}
	if out.Node.IsMount() {
		srcDriveID, err := out.Node.ReadMount()
		if err != nil {
			return PartialResolution{}, err
		}
		return PartialResolution{DriveID: srcDriveID, Path: out.Remaining}, nil
	}
	return PartialResolution{DriveID: driveID, Path: path}, nil
}

func (s *service) Lstat(ctx context.Context, driveID, path string) (Resolved, error) {
	d, n, err := s.resolveWithMounts(ctx, driveID, path, false)
	if err != nil {
		return Resolved{}, err
	}
	if n == nil {
		return Resolved{}, errorx.New(errorx.KindNotFound, "vfs: not found")
	}
	return Resolved{DriveID: d, Node: n}, nil
}

func (s *service) GetRootNodeID(ctx context.Context, driveID string) (uuid.UUID, error) {
	d, err := s.DriveClient.GetByID(ctx, driveID)
	if err != nil {
		return uuid.Nil, errorx.Wrap(err, fmt.Sprintf("vfs: get root node (drive_id=%s)", driveID))
	}
	if d == nil || d.RootNodeID() == nil {
		return uuid.Nil, errorx.New(errorx.KindNotFound, "vfs: not found")
	}
	return *d.RootNodeID(), nil
}

func (s *service) ResolveParentNodeID(ctx context.Context, driveID, path string) (uuid.UUID, string, error) {
	rootID, err := s.GetRootNodeID(ctx, driveID)
	if err != nil {
		return uuid.Nil, "", err
	}
	parent, name, err := newResolver(s.NodeClient).resolveParent(ctx, rootID, path)
	if err != nil {
		return uuid.Nil, "", err
	}
	if parent == nil {
		return uuid.Nil, "", errorx.New(errorx.KindNotFound, "vfs: not found")
	}
	return parent.ID(), name, nil
}

func (s *service) ResolveNodeID(ctx context.Context, driveID, path string) (uuid.UUID, error) {
	rootID, err := s.GetRootNodeID(ctx, driveID)
	if err != nil {
		return uuid.Nil, err
	}
	out, err := newResolver(s.NodeClient).resolvePath(ctx, rootID, path, true)
	if err != nil {
		return uuid.Nil, err
	}
	if out.Node == nil {
		return uuid.Nil, errorx.New(errorx.KindNotFound, "vfs: not found")
	}
	return out.Node.ID(), nil
}
