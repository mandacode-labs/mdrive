package vfs

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/internal/upload"
)

var (
	_ NodeClient  = (*node.Service)(nil)
	_ DriveClient = (*drive.Service)(nil)
	_ UserClient  = (*user.Service)(nil)
	_ PermClient  = (*permission.OpenFGAChecker)(nil)
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

type DriveClient interface {
	Create(ctx context.Context, name string, desc *string, ownerID string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error)
	GetByID(ctx context.Context, id string) (*drive.Drive, error)
	GetByPublicID(ctx context.Context, pubID string) (*drive.Drive, error)
	GetStorage(ctx context.Context, driveID string) (*drive.Storage, error)
	Update(ctx context.Context, id string, name, description *string) (*drive.Drive, error)
	Delete(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) (*drive.Drive, error)
	ListDeleted(ctx context.Context, before time.Time, limit int) ([]*drive.Drive, error)
	ListByOwner(ctx context.Context, ownerID string) ([]*drive.Drive, error)
}

type UserClient interface {
	UpsertFromOIDC(ctx context.Context, cmd *user.CreateCommand) (*user.User, error)
	GetByID(ctx context.Context, id string) (*user.User, error)
}

type PermClient interface {
	Check(ctx context.Context, userID string, perm permission.Permission, objType, objID string) (bool, error)
	Grant(ctx context.Context, userID, relation, objType, objID string) error
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
	User  UserClient
	Store Store
	Perm  PermClient
	Reg   upload.Registry
	GC    TombstoneInserter
}

func NewService(
	n NodeClient,
	d DriveClient,
	u UserClient,
	store Store,
	checker PermClient,
	reg upload.Registry,
	gc TombstoneInserter,
) *Service {
	if reg == nil {
		reg = upload.NewMemoryRegistry()
	}
	return &Service{
		Node:  n,
		Drive: d,
		User:  u,
		Store: store,
		Perm:  checker,
		Reg:   reg,
		GC:    gc,
	}
}

// newResolver returns a fresh resolver backed by the Service's
// NodeClient. Use it within an operation that resolves the same
// UUID more than once so the cache collapses the loads to a single
// *Node pointer; single-load callers can ignore the return value.
func (s *Service) newResolver() *resolver {
	return newResolver(s.Node)
}

func (s *Service) WithNodeTx(ctx context.Context, fn func(tx *Service) error) error {
	return s.Node.WithTx(ctx, func(txNode *node.Service) error {
		return fn(&Service{
			Node:  txNode,
			Drive: s.Drive,
			User:  s.User,
			Store: s.Store,
			Perm:  s.Perm,
			Reg:   s.Reg,
			GC:    s.GC,
		})
	})
}

type Resolved struct {
	DriveID string
	Node    *node.Node
}

const maxMountHops = 32

func (s *Service) Resolve(ctx context.Context, driveID, path string) (Resolved, error) {
	drive, n, err := s.resolve(ctx, driveID, path)
	if err != nil {
		return Resolved{}, err
	}
	return Resolved{DriveID: drive, Node: n}, nil
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

func (s *Service) checkAccess(ctx context.Context, userID string, perm permission.Permission, driveID string) error {
	if s.Perm == nil {
		return nil
	}
	allowed, err := s.Perm.Check(ctx, userID, perm, "drive", driveID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrPermission
	}
	return nil
}
