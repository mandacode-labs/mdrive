package vfs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
)

var (
	_ NodeClient  = (*node.Service)(nil)
	_ DriveClient = (*drive.Service)(nil)
)

// NodeClient is the subset of node.Service methods vfs needs to
// orchestrate the inode tree. vfs does not create nodes (the
// type-specific factories NewFile/NewDirectory/... in core/node
// do that, called by the vfs op entry points after the path
// has been resolved). vfs links, unlinks, moves, saves, and
// looks up by ID — that's it.
type NodeClient interface {
	Link(ctx context.Context, parent *node.Node, name string, child *node.Node) error
	Unlink(ctx context.Context, parent *node.Node, name string) (*node.Node, error)
	MoveEntry(ctx context.Context, srcParent *node.Node, srcName string, dstParent *node.Node, dstName string) error
	GetByID(ctx context.Context, id uuid.UUID) (*node.Node, error)
	Save(ctx context.Context, n *node.Node) error
}

// DriveClient is the data-access contract vfs needs from a drive
// service. vfs only needs read paths; permission checks are
// the caller's responsibility.
type DriveClient interface {
	GetByID(ctx context.Context, id string) (*drive.Drive, error)
	GetByPublicID(ctx context.Context, pubID string) (*drive.Drive, error)
	GetStorage(ctx context.Context, driveID string) (*drive.Storage, error)
}

// GarbageRef points at an external (S3) object that needs cleanup
// when an inode is removed. vfs only knows that something must
// eventually delete this object; the actual S3 call is owned by
// upload.Service, which implements GarbageRecorder.
type GarbageRef struct {
	Bucket string
	Key    string
}

// GarbageRecorder is the consumer-declared interface for marking
// external objects as garbage. vfs calls it when an inode's nlink
// hits 0; the implementation (typically upload.Service) writes a
// tombstone row that the gc.TombstoneCleaner job later drains.
//
// A nil GarbageRecorder means vfs will return an error if a
// tombstone would have been recorded. Production wires a real
// implementation; tests can leave it nil and only call paths that
// never produce tombstones.
type GarbageRecorder interface {
	RecordGarbage(ctx context.Context, refs []GarbageRef) error
}

type Service struct {
	Node  NodeClient
	Drive DriveClient
	// Garbage records external objects (S3) whose inode was
	// removed. nil is tolerated only when no vfs op produces
	// tombstones (e.g. tests with no object nodes).
	Garbage GarbageRecorder
	// Logger receives structured observability events for
	// multi-step filesystem operations (mount traversal, GC
	// tombstones, symlink cycles). Optional: nil means no-op.
	Logger *slog.Logger
}

// ServiceConfig groups the dependencies of NewService. vfs is
// filesystem-only: it has no user or permission dependencies.
// Permission checks are the caller's responsibility (handler layer).
type ServiceConfig struct {
	Node    NodeClient
	Drive   DriveClient
	Garbage GarbageRecorder
	Logger  *slog.Logger
}

func NewService(cfg ServiceConfig) *Service {
	return &Service{
		Node:    cfg.Node,
		Drive:   cfg.Drive,
		Garbage: cfg.Garbage,
		Logger:  cfg.Logger,
	}
}

// log returns the Service's logger. If none was configured (the
// common case for tests that do not assert on log output) a
// DiscardHandler is returned so the call sites can log
// unconditionally without polluting the test output. Production
// code wires a real logger in app.New.
func (s *Service) log() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.New(slog.DiscardHandler)
}

// newResolver returns a fresh resolver backed by the Service's
// NodeClient. The cache is shared across calls within the same
// operation (multiple resolves of the same UUID return the same
// *Node pointer).
func (s *Service) newResolver() *resolver {
	return newResolver(s.Node)
}

type Resolved struct {
	DriveID string
	Node    *node.Node
}

const maxMountHops = 32

// Resolve walks from root to the node at the given absolute path,
// Resolve walks the path following symlinks (POSIX stat(2)
// semantics) and transparently following mount nodes into other
// drives. Permission checking is the caller's responsibility:
// Resolve itself only does path resolution. Callers that need a
// permission check should use Resolve and then check against
// Resolved.DriveID.
func (s *Service) Resolve(ctx context.Context, driveID, path string) (Resolved, error) {
	drive, n, err := s.resolveCross(ctx, driveID, path, true)
	if err != nil {
		return Resolved{}, err
	}
	return Resolved{DriveID: drive, Node: n}, nil
}

// ResolvedRef is the partial resolution returned by ResolveForPermission:
// the drive the path lands in (after stopping at the first mount) and
// the remaining path within that drive.
type ResolvedRef struct {
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
func (s *Service) ResolveForPermission(ctx context.Context, driveID, path string) (ResolvedRef, error) {
	r := s.newResolver()
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return ResolvedRef{}, err
	}
	out, err := r.resolve(ctx, rootID, path, true)
	if err != nil {
		return ResolvedRef{}, err
	}
	if out.Node.IsMount() {
		srcDriveID, err := out.Node.ReadMount()
		if err != nil {
			return ResolvedRef{}, err
		}
		return ResolvedRef{DriveID: srcDriveID, Path: out.Remaining}, nil
	}
	return ResolvedRef{DriveID: driveID, Path: path}, nil
}

// Lstat is the standalone no-symlink-follow variant. It returns
// the final node without traversing symlinks (POSIX lstat(2)).
// Mount traversal still happens; only the symlink follow is
// skipped. Used by callers that need the resolved node itself
// (e.g. the readlink handler).
func (s *Service) Lstat(ctx context.Context, driveID, path string) (Resolved, error) {
	d, n, err := s.resolveCross(ctx, driveID, path, false)
	if err != nil {
		return Resolved{}, err
	}
	return Resolved{DriveID: d, Node: n}, nil
}

func (s *Service) rootNodeID(ctx context.Context, driveID string) (uuid.UUID, error) {
	d, err := s.Drive.GetByID(ctx, driveID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("vfs: %w", err)
	}
	if d == nil || d.RootNodeID() == nil {
		return uuid.Nil, ErrNotFound
	}
	return *d.RootNodeID(), nil
}

// GetRootNodeID is the public form of rootNodeID. External services
// (notably internal/upload.Service) use it to resolve a drive's
// root node for path operations.
func (s *Service) GetRootNodeID(ctx context.Context, driveID string) (uuid.UUID, error) {
	return s.rootNodeID(ctx, driveID)
}

// ResolveParentNodeID resolves a path's parent directory and last
// component, returning the parent's node ID and the leaf name.
// Public so external services (internal/upload.Service) can link
// new nodes without taking on the resolver.
func (s *Service) ResolveParentNodeID(ctx context.Context, driveID, path string) (uuid.UUID, string, error) {
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return uuid.Nil, "", err
	}
	parent, name, err := s.newResolver().resolveParent(ctx, rootID, path)
	if err != nil {
		return uuid.Nil, "", err
	}
	return parent.ID(), name, nil
}

// ResolveNodeID resolves a path to its node ID within a drive.
// Public so external services (internal/upload.Service) can fetch
// an existing node without a full Resolved struct.
func (s *Service) ResolveNodeID(ctx context.Context, driveID, path string) (uuid.UUID, error) {
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return uuid.Nil, err
	}
	out, err := s.newResolver().resolve(ctx, rootID, path, true)
	if err != nil {
		return uuid.Nil, err
	}
	return out.Node.ID(), nil
}
