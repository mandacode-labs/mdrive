package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Service is the VFS orchestration layer.
//
// It exposes POSIX-style file system commands (Mkdir, Touch, Rm, Mv, Ls, Cat, etc.)
// that operate on path strings. Under the hood it composes node/drive/user domain
// services with a Storage backend and permission checks via OpenFGA.
type Service struct {
	nodeSvc  *node.Service
	driveSvc *drive.Service
	userSvc  *user.Service
	store    Storage
	perm     permission.Checker
	path     *Resolver
}

// NewService creates a new VFS Service.
func NewService(
	nodeSvc *node.Service,
	driveSvc *drive.Service,
	userSvc *user.Service,
	store Storage,
	checker permission.Checker,
) *Service {
	return &Service{
		nodeSvc:  nodeSvc,
		driveSvc: driveSvc,
		userSvc:  userSvc,
		store:    store,
		perm:     checker,
		path:     newResolver(nodeSvc),
	}
}

// WithTx executes fn within a transaction scoped to the node domain.
func (s *Service) WithTx(ctx context.Context, fn func(tx *Service) error) error {
	return s.nodeSvc.WithTx(ctx, func(txNode *node.Service) error {
		return fn(&Service{
			nodeSvc:  txNode,
			driveSvc: s.driveSvc,
			userSvc:  s.userSvc,
			store:    s.store,
			perm:     s.perm,
			path:     newResolver(txNode),
		})
	})
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

// mustGetDrive returns the drive or panics (internal helper for already-valid call chains).
func (s *Service) mustGetDrive(ctx context.Context, driveID string) *drive.Drive {
	d, err := s.driveSvc.GetByID(ctx, driveID)
	if err != nil || d == nil || d.RootNodeID() == nil {
		return nil
	}
	return d
}
