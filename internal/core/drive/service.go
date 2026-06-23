package drive

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Service provides domain-level drive operations. It is the single
// canonical drive service: permission checks are integrated via
// the Perm field. Callers (handler, vfs) do not need a separate
// "drive.Service" wrapper to add permission semantics.
//
// Permission is checked in Create, Update, Delete, and Restore.
// ListDeleted requires an explicit isAdmin flag (the handler
// sources this from the session admin role). ListByOwner and
// GetByID/PublicID are by-design unprotected — the handler
// decides whether the caller may see another user's data.
//
// A nil Perm (development mode) is tolerated; all permission
// checks become no-ops.
type Service struct {
	repo       Repository
	users      Exister
	rootCreate RootCreator
	perm       permission.Checker
}

// NewService creates a new Service. Perm is optional; pass nil in
// development to skip all permission checks.
func NewService(repo Repository, users Exister, rootCreate RootCreator, perm permission.Checker) *Service {
	return &Service{repo: repo, users: users, rootCreate: rootCreate, perm: perm}
}

// Create creates a drive and its root directory node. The drive +
// storage rows and the drive's root-node update run inside a single
// repository transaction so partial failure cannot leave a drive
// record pointing at a non-existent root ID.
//
// The root node itself is created by an external RootCreator
// (typically the node.Service wired by the CLI). That step spans
// repositories, so it cannot participate in this tx. The orphan-
// cleanup path (gc/cli) covers any root node that ends up without
// a matching drive row.
//
// actorID is both the owner of the new drive and the principal
// on whose behalf the owner permission is granted; callers must
// pass the authenticated user.
func (s *Service) Create(ctx context.Context, actorID string, name, description string, cfg StorageConfig) (*Drive, uuid.UUID, error) {
	if name == "" {
		return nil, uuid.Nil, ErrInvalidName
	}
	if actorID == "" {
		return nil, uuid.Nil, fmt.Errorf("drive: owner_id is required")
	}
	if !storageCfgValid(&cfg) {
		return nil, uuid.Nil, ErrInvalidCredentials
	}

	exists, err := s.users.Exists(ctx, actorID)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("check owner: %w", err)
	}
	if !exists {
		return nil, uuid.Nil, fmt.Errorf("drive: owner not found")
	}

	id := ulid.Make().String()
	now := time.Now()
	var descPtr *string
	if description != "" {
		descPtr = &description
	}
	d := NewDrive(id, id, name, descPtr, ProviderS3, actorID, nil, nil, now, now)
	s2 := NewStorage(id, cfg.Bucket, cfg.Endpoint, cfg.Region,
		cfg.AccessKey, cfg.SecretKey, cfg.UsePathStyle)

	rootID, err := s.rootCreate.NewRootDirectory(ctx)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("create root node: %w", err)
	}
	d.SetRootNodeID(rootID)

	var updated *Drive
	err = s.WithTx(ctx, func(tx *Service) error {
		if err := tx.repo.Create(ctx, d, s2); err != nil {
			return fmt.Errorf("create drive: %w", err)
		}
		u, err := tx.repo.Update(ctx, d)
		if err != nil {
			return fmt.Errorf("update drive with root: %w", err)
		}
		updated = u
		return nil
	})
	if err != nil {
		return nil, uuid.Nil, err
	}
	return updated, rootID, nil
}

// Get is the permission-checked alias of GetByID. The actorID
// is currently unused (GetByID is by-design unprotected — the
// handler decides whether the caller may see a drive), but the
// parameter exists to keep the call site uniform with the
// permission-bearing methods.
func (s *Service) Get(ctx context.Context, _, id string) (*Drive, error) {
	return s.GetByID(ctx, id)
}

// GetByID returns a drive by its private ID.
func (s *Service) GetByID(ctx context.Context, id string) (*Drive, error) {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, ErrNotFound
	}
	return d, nil
}

// GetByPublicID returns a drive by its public ID.
func (s *Service) GetByPublicID(ctx context.Context, publicID string) (*Drive, error) {
	d, err := s.repo.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, ErrNotFound
	}
	return d, nil
}

// GetStorage returns the storage configuration for a drive.
// The actorID is currently unused (storage config contains
// credentials; the handler's permission policy is to gate
// access to the drive, not the storage record), but the
// parameter exists to keep the call site uniform with the
// permission-bearing methods.
func (s *Service) GetStorage(ctx context.Context, _, driveID string) (*Storage, error) {
	st, err := s.repo.GetStorage(ctx, driveID)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, ErrNotFound
	}
	return st, nil
}

// Update updates mutable drive fields. Requires edit permission.
// Empty name or description means "leave unchanged"; pass an
// explicit value to override.
func (s *Service) Update(ctx context.Context, actorID, id string, name, description string) (*Drive, error) {
	if err := s.requirePerm(ctx, actorID, permission.PermissionEdit, id); err != nil {
		return nil, err
	}
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, ErrNotFound
	}
	newName := d.Name()
	if name != "" {
		newName = name
	}
	newDesc := d.Description()
	if description != "" {
		newDesc = &description
	}
	updated := NewDrive(
		d.ID(), d.PublicID(), newName, newDesc,
		d.Provider(), d.OwnerID(), d.RootNodeID(), d.DeletedAt(),
		d.CreatedAt(), time.Now(),
	)
	return s.repo.Update(ctx, updated)
}

// Delete soft-deletes a drive. Requires delete permission.
func (s *Service) Delete(ctx context.Context, actorID, id string) error {
	if err := s.requirePerm(ctx, actorID, permission.PermissionDelete, id); err != nil {
		return err
	}
	if _, err := s.GetByID(ctx, id); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, id)
}

// Restore reactivates a soft-deleted drive. Requires manage
// permission; the handler additionally gates on session admin.
func (s *Service) Restore(ctx context.Context, actorID, id string) (*Drive, error) {
	if err := s.requirePerm(ctx, actorID, permission.PermissionManage, id); err != nil {
		return nil, err
	}
	d, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d.DeletedAt() == nil {
		return nil, fmt.Errorf("drive: not deleted")
	}
	if err := s.repo.Restore(ctx, id); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// Purge permanently removes a soft-deleted drive and its storage record.
func (s *Service) Purge(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// ListDeletedForAdmin returns drives soft-deleted before the given
// time, limited. Admin-only; the service trusts isAdmin.
//
// Handler callers should use the higher-level ListDeletedDrives
// pattern (see internal/app/apiserver/handler/drive.go).
func (s *Service) ListDeletedForAdmin(ctx context.Context, isAdmin bool, before time.Time, limit int) ([]*Drive, error) {
	if !isAdmin {
		return nil, permission.ErrPermission
	}
	return s.repo.FindDeleted(ctx, before, limit)
}

// ListDeletedByOwner returns soft-deleted drives for a specific owner.
func (s *Service) ListDeletedByOwner(ctx context.Context, actorID string) ([]*Drive, error) {
	return s.repo.FindDeletedByOwner(ctx, actorID)
}

// ListByOwner returns all drives owned by actorID.
func (s *Service) ListByOwner(ctx context.Context, actorID string) ([]*Drive, error) {
	return s.repo.FindByOwner(ctx, actorID)
}

// WithTx executes fn within a transaction.
func (s *Service) WithTx(ctx context.Context, fn func(*Service) error) error {
	return s.repo.WithTx(ctx, func(txRepo Repository) error {
		return fn(&Service{repo: txRepo, users: s.users, rootCreate: s.rootCreate, perm: s.perm})
	})
}

// requirePerm is the canonical permission check. Nil perm
// (development mode) is tolerated.
func (s *Service) requirePerm(ctx context.Context, actorID string, perm permission.Permission, driveID string) error {
	return permission.Require(ctx, s.perm, actorID, perm, permission.ObjectTypeDrive, driveID)
}

func storageCfgValid(cfg *StorageConfig) bool {
	return cfg.Bucket != "" && cfg.Region != ""
}
