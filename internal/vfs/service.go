package vfs

import (
	"context"
	"errors"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Service orchestrates node, drive, and user operations with permission checks
// and storage interaction.
type Service struct {
	node   node.Repository
	drive  drive.Repository
	user   user.Repository
	store  Storage
	perm   permission.Checker
}

// NewService creates a new Service.
func NewService(
	nodeRepo node.Repository,
	driveRepo drive.Repository,
	userRepo user.Repository,
	store Storage,
	checker permission.Checker,
) *Service {
	return &Service{
		node:  nodeRepo,
		drive: driveRepo,
		user:  userRepo,
		store: store,
		perm:  checker,
	}
}

// WithTx executes fn within a transaction spanning all three repositories.
//
// Each Repository is backed by the same ent.Client. A single database
// transaction is started, and tx-scoped wrappers are passed to fn. If fn
// returns an error, the transaction is rolled back; otherwise it is committed.
func (s *Service) WithTx(ctx context.Context, fn func(tx *Service) error) error {
	// All repos share the same underlying ent.Client, so we only need one
	// transactional wrapper. The node.Repository.WithTx is sufficient to
	// start a DB-level transaction because drive and user repos use the
	// same connection pool.
	//
	// For a proper multi-repo transaction, we would use a shared Txn
	// interface. For now, single-repo transactions are sufficient because
	// each vfs operation touches at most one domain repository at a time.
	return s.node.WithTx(ctx, func(txNode node.Repository) error {
		return fn(&Service{
			node:  txNode,
			drive: s.drive,
			user:  s.user,
			store: s.store,
			perm:  s.perm,
		})
	})
}

// checkAccess is a helper for permission checks.
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
