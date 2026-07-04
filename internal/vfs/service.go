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

var (
	_ node.Linker  = (*node.Service)(nil)
	_ drive.Reader = (*drive.Service)(nil)
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

type Service struct {
	NodeClient      node.Linker
	DriveClient     drive.Reader
	GarbageRecorder GarbageRecorder
	tm              entx.TxManager
}

// ServiceConfig groups the dependencies of NewService. Permission
// checks are the caller's responsibility.
type ServiceConfig struct {
	NodeClient      node.Linker
	DriveClient     drive.Reader
	GarbageRecorder GarbageRecorder
	TxManager       entx.TxManager
}

func NewService(cfg ServiceConfig) *Service {
	return &Service{
		NodeClient:      cfg.NodeClient,
		DriveClient:     cfg.DriveClient,
		GarbageRecorder: cfg.GarbageRecorder,
		tm:              cfg.TxManager,
	}
}

type Resolved struct {
	DriveID string
	Node    *node.Node
}

const maxMountHops = 32

// Resolve walks the path from root to the node, following
// symlinks (POSIX stat(2) semantics) and transparently following
// mount nodes into other drives. Permission checking is the
// caller's responsibility: Resolve itself only does path
// resolution. Callers that need a permission check should use
// Resolve and then check against Resolved.DriveID.
func (s *Service) Resolve(ctx context.Context, driveID, path string) (Resolved, error) {
	drive, n, err := s.resolveWithMounts(ctx, driveID, path, true)
	if err != nil {
		return Resolved{}, err
	}
	if n == nil {
		return Resolved{}, errorx.New(errorx.KindNotFound, "vfs: not found")
	}
	return Resolved{DriveID: drive, Node: n}, nil
}

// PartialResolution is the partial resolution returned by ResolveForPermission:
// the drive the path lands in (after stopping at the first mount) and
// the remaining path within that drive.
type PartialResolution struct {
	DriveID string
	Path    string
}

// ResolveForPermission is the variant of Resolve for callers that
// need to perform a permission check on the resolved drive (the
// one the path actually lands in, not the requested driveID). It
// stops at the first mount node and returns the drive to which the
// mount points plus the remaining path within that drive, so the
// caller can both check permission and then re-resolve the rest of
// the path within the source drive. Symlinks are followed.
func (s *Service) ResolveForPermission(ctx context.Context, driveID, path string) (PartialResolution, error) {
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

// Lstat is the standalone no-symlink-follow variant. It returns
// the final node without traversing symlinks (POSIX lstat(2)).
// Mount traversal still happens; only the symlink follow is
// skipped. Used by callers that need the resolved node itself
// (e.g. the readlink handler).
func (s *Service) Lstat(ctx context.Context, driveID, path string) (Resolved, error) {
	d, n, err := s.resolveWithMounts(ctx, driveID, path, false)
	if err != nil {
		return Resolved{}, err
	}
	if n == nil {
		return Resolved{}, errorx.New(errorx.KindNotFound, "vfs: not found")
	}
	return Resolved{DriveID: d, Node: n}, nil
}

// GetRootNodeID resolves a drive's root node ID. External services
// (notably internal/upload.Service) use it to anchor path
// operations.
func (s *Service) GetRootNodeID(ctx context.Context, driveID string) (uuid.UUID, error) {
	d, err := s.DriveClient.GetByID(ctx, driveID)
	if err != nil {
		return uuid.Nil, errorx.Wrap(err, fmt.Sprintf("vfs: get root node (drive_id=%s)", driveID))
	}
	if d == nil || d.RootNodeID() == nil {
		return uuid.Nil, errorx.New(errorx.KindNotFound, "vfs: not found")
	}
	return *d.RootNodeID(), nil
}

// ResolveParentNodeID resolves a path's parent directory and last
// component, returning the parent's node ID and the leaf name.
// Public so external services (internal/upload.Service) can link
// new nodes without taking on the resolver.
func (s *Service) ResolveParentNodeID(ctx context.Context, driveID, path string) (uuid.UUID, string, error) {
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

// ResolveNodeID resolves a path to its node ID within a drive.
// Public so external services (internal/upload.Service) can fetch
// an existing node without a full Resolved struct.
func (s *Service) ResolveNodeID(ctx context.Context, driveID, path string) (uuid.UUID, error) {
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
