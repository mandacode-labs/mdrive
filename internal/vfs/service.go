package vfs

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Compile-time interface satisfaction: core services satisfy vfs-declared interfaces.
var (
	_ nodeClient = (*node.Service)(nil)
	_ driveClient = (*drive.Service)(nil)
	_ userClient = (*user.Service)(nil)
	_ permClient = (*permission.OpenFGAChecker)(nil)
)

// --------------- Consumer-declared interfaces ---------------

type nodeClient interface {
	CreateFile(ctx context.Context, content string) (*node.Node, error)
	CreateDirectory(ctx context.Context) (*node.Node, error)
	CreateSymlink(ctx context.Context, target string) (*node.Node, error)
	CreateObject(ctx context.Context, content node.ObjectContent, size int64) (*node.Node, error)
	Link(ctx context.Context, parent *node.Node, name string, child *node.Node) error
	Unlink(ctx context.Context, parent *node.Node, name string) error
	GetByID(ctx context.Context, id uuid.UUID) (*node.Node, error)
	Delete(ctx context.Context, id uuid.UUID) error
	WithTx(ctx context.Context, fn func(tx *node.Service) error) error
}

type driveClient interface {
	Create(ctx context.Context, name string, desc *string, ownerID string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error)
	GetByID(ctx context.Context, id string) (*drive.Drive, error)
	GetByPublicID(ctx context.Context, pubID string) (*drive.Drive, error)
	GetStorage(ctx context.Context, driveID string) (*drive.Storage, error)
	Update(ctx context.Context, id string, name, description *string) (*drive.Drive, error)
	Delete(ctx context.Context, id string) error
	ListByOwner(ctx context.Context, ownerID string) ([]*drive.Drive, error)
}

type userClient interface {
	UpsertFromOIDC(ctx context.Context, cmd *user.CreateCommand) (*user.User, error)
	GetByID(ctx context.Context, id string) (*user.User, error)
	GetByPublicID(ctx context.Context, pubID string) (*user.User, error)
	GetByProviderID(ctx context.Context, provider, providerID string) (*user.User, error)
	Update(ctx context.Context, u *user.User) (*user.User, error)
	Exists(ctx context.Context, id string) (bool, error)
}

type permClient interface {
	Check(ctx context.Context, userID, relation, objType, objID string) (bool, error)
	Grant(ctx context.Context, userID, relation, objType, objID string) error
}

// --------------- VFS Service ---------------

// Service is the VFS orchestration layer.
type Service struct {
	node  nodeClient
	drive driveClient
	user  userClient
	store Storage
	perm  permClient
	path  *resolver
}

// NewService creates a new VFS Service.
func NewService(
	n nodeClient,
	d driveClient,
	u userClient,
	store Storage,
	checker permClient,
) *Service {
	return &Service{
		node:  n,
		drive: d,
		user:  u,
		store: store,
		perm:  checker,
		path:  newResolver(n),
	}
}

// WithTx executes fn within a node-domain transaction.
func (s *Service) WithTx(ctx context.Context, fn func(tx *Service) error) error {
	return s.node.WithTx(ctx, func(txNode *node.Service) error {
		return fn(&Service{
			node:  txNode,
			drive: s.drive,
			user:  s.user,
			store: s.store,
			perm:  s.perm,
			path:  newResolver(txNode),
		})
	})
}

// rootNodeID resolves the root node UUID for the given drive.
func (s *Service) rootNodeID(ctx context.Context, driveID string) (uuid.UUID, error) {
	d, err := s.drive.GetByID(ctx, driveID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("vfs: %w", err)
	}
	if d == nil || d.RootNodeID() == nil {
		return uuid.Nil, ErrNotFound
	}
	return *d.RootNodeID(), nil
}

// checkAccess returns nil if the user has the given permission on the drive.
func (s *Service) checkAccess(ctx context.Context, userID, permission, driveID string) error {
	allowed, err := s.perm.Check(ctx, userID, permission, "drive", driveID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrPermission
	}
	return nil
}
