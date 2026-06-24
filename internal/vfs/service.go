package vfs

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
)

var (
	_ NodeClient  = (*node.Service)(nil)
	_ DriveClient = (*drive.Service)(nil)
)

type NodeClient interface {
	CreateFile(ctx context.Context, content string) (*node.Node, error)
	Touch(ctx context.Context) (*node.Node, error)
	CreateDirectory(ctx context.Context) (*node.Node, error)
	CreateSymlink(ctx context.Context, target string) (*node.Node, error)
	CreateObject(ctx context.Context, content node.ObjectContent, size int64) (*node.Node, error)
	CreateMount(ctx context.Context, sourceDriveID string) (*node.Node, error)
	Link(ctx context.Context, parent *node.Node, name string, child *node.Node) error
	BulkLink(ctx context.Context, parent *node.Node, entries map[string]*node.Node) error
	Unlink(ctx context.Context, parent *node.Node, name string) (*node.Node, error)
	BulkUnlink(ctx context.Context, parent *node.Node, names []string) ([]*node.Node, error)
	UnlinkOrReplace(ctx context.Context, parent *node.Node, name string) (*node.Node, error)
	MoveEntry(ctx context.Context, srcParent *node.Node, srcName string, dstParent *node.Node, dstName string) error
	GetByID(ctx context.Context, id uuid.UUID) (*node.Node, error)
	Save(ctx context.Context, n *node.Node) error
	Delete(ctx context.Context, id uuid.UUID) error
	WithTx(ctx context.Context, fn func(tx *node.Service) error) error
}

// DriveClient is the data-access contract vfs needs from a drive
// service. vfs only needs read paths. The actorID parameter is
// unused for these read methods (by-design unprotected: the
// handler decides whether the caller may see the data), but the
// interface is shaped to the underlying drive service so a real
// *drive.Service satisfies it.
type DriveClient interface {
	GetByID(ctx context.Context, id string) (*drive.Drive, error)
	GetByPublicID(ctx context.Context, pubID string) (*drive.Drive, error)
	GetStorage(ctx context.Context, actorID, driveID string) (*drive.Storage, error)
}

type TombstoneInserter interface {
	InsertTombstones(ctx context.Context, refs []ObjectRef) error
}

type ObjectRef struct {
	Bucket string
	Key    string
}

type Service struct {
	Node  NodeClient
	Drive DriveClient
	Store Store
	GC    TombstoneInserter
}

// ServiceConfig groups the dependencies of NewService. vfs is
// filesystem-only: it has no user or permission dependencies.
// Permission checks are the caller's responsibility (handler layer).
type ServiceConfig struct {
	Node  NodeClient
	Drive DriveClient
	Store Store
	GC    TombstoneInserter
}

func NewService(cfg ServiceConfig) *Service {
	return &Service{
		Node:  cfg.Node,
		Drive: cfg.Drive,
		Store: cfg.Store,
		GC:    cfg.GC,
	}
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

// ResolveForPermission is the variant of Resolve for callers that
// need to perform a permission check on the resolved drive (the
// one the path actually lands in, not the requested driveID). It
// stops at the first mount node and returns the drive to which the
// mount points plus the remaining path within that drive, so the
// caller can both check permission and then re-resolve the rest of
// the path within the source drive. Symlinks are followed.
type ResolvedRef struct {
	DriveID string
	Path    string
}

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
	r := s.newResolver()
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return Resolved{}, err
	}
	out, err := r.resolve(ctx, rootID, path, false)
	if err != nil {
		return Resolved{}, err
	}
	if out.Remaining != "" {
		// Path crossed a mount: re-resolve inside the source
		// drive (still no symlink follow).
		srcDriveID, err := out.Node.ReadMount()
		if err != nil {
			return Resolved{}, err
		}
		drive2, n2, err := s.resolveCross(ctx, srcDriveID, out.Remaining, false)
		if err != nil {
			return Resolved{}, err
		}
		return Resolved{DriveID: drive2, Node: n2}, nil
	}
	return Resolved{DriveID: driveID, Node: out.Node}, nil
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
