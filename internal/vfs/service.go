package vfs

import (
	"context"
	"errors"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Service is the VFS orchestration layer. It composes domain services
// (node, drive, user) with cross-cutting concerns (Storage, Permission)
// to perform path-based, permission-checked operations.
type Service struct {
	nodeSvc  *node.Service
	driveSvc *drive.Service
	userSvc  *user.Service
	store    Storage
	perm     permission.Checker
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
		return errors.New("vfs: permission denied")
	}
	return nil
}
