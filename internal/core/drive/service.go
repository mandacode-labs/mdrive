package drive

import (
	"fmt"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Service provides domain-level drive operations. Permission
// checks are the caller's responsibility (the handler layer);
// drive.Service is the pure domain logic. ListDeletedForAdmin
// still accepts an isAdmin flag — that gate is the *administrative*
// role, not a per-drive permission check, and the handler
// sources it from the session.
type Service struct {
	repo                 Repository
	ownerChecker          OwnerChecker
	rootDirectoryCreator RootDirectoryCreator
}

// NewService creates a new Service. Permission checks live in the
// handler; the service is the pure domain layer.
func NewService(repo Repository, ownerChecker OwnerChecker, rootDirectoryCreator RootDirectoryCreator) *Service {
	return &Service{repo: repo, ownerChecker: ownerChecker, rootDirectoryCreator: rootDirectoryCreator}
}

// Create creates a drive and its root directory node. The drive +
// storage rows and the drive's root-node update run inside a single
// repository transaction so partial failure cannot leave a drive
// record pointing at a non-existent root ID.
//
// The root node itself is created by an external RootDirectoryCreator
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
		return nil, uuid.Nil, errorx.New(errorx.KindBadRequest, "drive: invalid name (name is empty)")
	}
	if actorID == "" {
		return nil, uuid.Nil, errorx.New(errorx.KindBadRequest, "drive: owner_id is required")
	}
	if err := storageCfgValid(&cfg); err != nil {
		return nil, uuid.Nil, errorx.Wrap(err, "drive: storage invalid")
	}

	exists, err := s.ownerChecker.Exist(ctx, actorID)
	if err != nil {
		return nil, uuid.Nil, errorx.Wrap(err, fmt.Sprintf("drive: owner check failed (actor_id=%s)", actorID), errorx.KindServiceDegraded)
	}
	if !exists {
		return nil, uuid.Nil, errorx.New(errorx.KindForbidden, "drive: owner not found (actor_id="+actorID+")")
	}

	id := ulid.Make().String()
	now := time.Now()
	var descriptionPtr *string
	if description != "" {
		descriptionPtr = &description
	}
	d := NewDrive(id, id, name, descriptionPtr, ProviderS3, actorID, nil, nil, now, now)
	storage := NewStorage(id, cfg.Bucket, cfg.Endpoint, cfg.Region,
		cfg.AccessKey, cfg.SecretKey, cfg.UsePathStyle)

	rootID, err := s.rootDirectoryCreator.CreateRootDirectory(ctx)
	if err != nil {
		return nil, uuid.Nil, errorx.Wrap(err, fmt.Sprintf("drive: root node creation failed (id=%s)", id), errorx.KindServiceDegraded)
	}
	d.SetRootNodeID(rootID)

	var updated *Drive
	err = s.WithTx(ctx, func(tx *Service) error {
		if err := tx.repo.Create(ctx, d, storage); err != nil {
			return errorx.Wrap(err, fmt.Sprintf("drive: repo create failed (id_len=%d, owner_id_len=%d, root_id=%s)", len(d.ID()), len(d.OwnerID()), rootID), errorx.KindServiceDegraded)
		}
		u, err := tx.repo.Update(ctx, d)
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("drive: repo update failed (id_len=%d)", len(d.ID())), errorx.KindServiceDegraded)
		}
		updated = u
		return nil
	})
	if err != nil {
		return nil, uuid.Nil, errorx.Wrap(err, fmt.Sprintf("drive: tx failed (id=%s, root_id=%s)", id, rootID), errorx.KindServiceDegraded)
	}
	return updated, rootID, nil
}

// GetByID returns a drive by its private ID. Permission is the
// caller's responsibility; the handler gates on view before
// reaching this method.
func (s *Service) GetByID(ctx context.Context, id string) (*Drive, error) {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, errorx.New(errorx.KindNotFound, "drive: not found (id="+id+")")
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
		return nil, errorx.New(errorx.KindNotFound, "drive: not found (public_id="+publicID+")")
	}
	return d, nil
}

// GetStorage returns the storage configuration for a drive.
// The handler gates this call with requirePerm; the service
// itself does not check ownership.
func (s *Service) GetStorage(ctx context.Context, driveID string) (*Storage, error) {
	st, err := s.repo.GetStorage(ctx, driveID)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, errorx.New(errorx.KindNotFound, "drive: storage not found (drive_id="+driveID+")")
	}
	return st, nil
}

// Update updates mutable drive fields. Permission is the
// caller's responsibility; the handler gates on edit.
//
// Empty name or description means "leave unchanged"; pass an
// explicit value to override.
func (s *Service) Update(ctx context.Context, id string, name, description string) (*Drive, error) {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, errorx.New(errorx.KindNotFound, "drive: not found (id="+id+")")
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

// Delete soft-deletes a drive. Permission is the caller's
// responsibility; the handler gates on delete.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.GetByID(ctx, id); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, id)
}

// Restore reactivates a soft-deleted drive. Permission is the
// caller's responsibility; the handler gates on manage + admin.
func (s *Service) Restore(ctx context.Context, id string) (*Drive, error) {
	d, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d.DeletedAt() == nil {
		return nil, errorx.New(errorx.KindBadRequest, "drive: not deleted (id="+id+")")
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
		return nil, errorx.New(errorx.KindForbidden, "permission: denied")
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
		return fn(&Service{repo: txRepo, ownerChecker: s.ownerChecker, rootDirectoryCreator: s.rootDirectoryCreator})
	})
}

func storageCfgValid(cfg *StorageConfig) error {
	if cfg.Bucket == "" {
		return errorx.New(errorx.KindBadRequest, "drive: storage bucket is required")
	}
	if cfg.Region == "" {
		return errorx.New(errorx.KindBadRequest, "drive: storage region is required")
	}
	return nil
}