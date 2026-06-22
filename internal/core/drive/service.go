package drive

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// DEKProvider supplies wrapped per-drive data encryption keys.
// The implementation lives in the crypto layer; the drive domain
// only depends on this small interface so the service can stay
// crypto-agnostic. NewWrappedDEK is called once per drive creation
// to produce the wrapped DEK that is persisted alongside the
// storage credentials.
type DEKProvider interface {
	NewWrappedDEK() (string, error)
}

// Service provides domain-level drive operations.
// It wraps Repository with validation and convenience, analogous to
// Linux super_operations (alloc_inode, destroy_inode) but without
// permission checks — those belong to vfs.
type Service struct {
	repo       Repository
	users      Exister
	rootCreate RootCreator
	dek        DEKProvider
}

// NewService creates a new Service. dek may be nil in dev/test where
// envelope encryption is not used; the service skips DEK
// provisioning in that case.
func NewService(repo Repository, users Exister, rootCreate RootCreator, dek DEKProvider) *Service {
	return &Service{repo: repo, users: users, rootCreate: rootCreate, dek: dek}
}

// Create creates a drive and its root directory node.
func (s *Service) Create(ctx context.Context, name string, desc *string, ownerID string, cfg StorageConfig) (*Drive, uuid.UUID, error) {
	if name == "" {
		return nil, uuid.Nil, ErrInvalidName
	}
	if ownerID == "" {
		return nil, uuid.Nil, fmt.Errorf("drive: owner_id is required")
	}
	if !storageCfgValid(&cfg) {
		return nil, uuid.Nil, ErrInvalidCredentials
	}

	exists, err := s.users.Exists(ctx, ownerID)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("check owner: %w", err)
	}
	if !exists {
		return nil, uuid.Nil, fmt.Errorf("drive: owner not found")
	}

	id := ulid.Make().String()
	now := time.Now()
	d := NewDrive(id, id, name, desc, ProviderS3, ownerID, nil, nil, now, now)

	var wrappedDEK string
	if s.dek != nil {
		wrappedDEK, err = s.dek.NewWrappedDEK()
		if err != nil {
			return nil, uuid.Nil, fmt.Errorf("generate dek: %w", err)
		}
	}

	s2 := NewStorage(id, cfg.Bucket, cfg.Endpoint, cfg.Region,
		cfg.AccessKey, cfg.SecretKey, cfg.UsePathStyle, wrappedDEK)

	if err := s.repo.Create(ctx, d, s2); err != nil {
		return nil, uuid.Nil, fmt.Errorf("create drive: %w", err)
	}

	rootID, err := s.rootCreate.NewRootDirectory(ctx)
	if err != nil {
		_ = s.repo.Delete(ctx, id)
		return nil, uuid.Nil, fmt.Errorf("create root node: %w", err)
	}
	d.SetRootNodeID(rootID)

	updated, err := s.repo.Update(ctx, d)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("update drive with root: %w", err)
	}
	return updated, rootID, nil
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
func (s *Service) GetStorage(ctx context.Context, driveID string) (*Storage, error) {
	st, err := s.repo.GetStorage(ctx, driveID)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, ErrNotFound
	}
	return st, nil
}

// Update updates mutable drive fields.
func (s *Service) Update(ctx context.Context, id string, name, description *string) (*Drive, error) {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, ErrNotFound
	}
	newName := d.Name()
	if name != nil {
		newName = *name
	}
	newDesc := d.Description()
	if description != nil {
		newDesc = description
	}
	updated := NewDrive(
		d.ID(), d.PublicID(), newName, newDesc,
		d.Provider(), d.OwnerID(), d.RootNodeID(), d.DeletedAt(),
		d.CreatedAt(), time.Now(),
	)
	return s.repo.Update(ctx, updated)
}

// Delete soft-deletes a drive.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.GetByID(ctx, id); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, id)
}

// Restore reactivates a soft-deleted drive.
func (s *Service) Restore(ctx context.Context, id string) (*Drive, error) {
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

// ListDeleted returns drives soft-deleted before the given time.
func (s *Service) ListDeleted(ctx context.Context, before time.Time, limit int) ([]*Drive, error) {
	return s.repo.FindDeleted(ctx, before, limit)
}

// ListDeletedByOwner returns soft-deleted drives for a specific owner.
func (s *Service) ListDeletedByOwner(ctx context.Context, ownerID string) ([]*Drive, error) {
	return s.repo.FindDeletedByOwner(ctx, ownerID)
}

// ListByOwner returns all drives owned by ownerID.
func (s *Service) ListByOwner(ctx context.Context, ownerID string) ([]*Drive, error) {
	return s.repo.FindByOwner(ctx, ownerID)
}

// WithTx executes fn within a transaction.
func (s *Service) WithTx(ctx context.Context, fn func(*Service) error) error {
	return s.repo.WithTx(ctx, func(txRepo Repository) error {
		return fn(&Service{repo: txRepo, users: s.users, rootCreate: s.rootCreate})
	})
}

func storageCfgValid(cfg *StorageConfig) bool {
	return cfg.Bucket != "" && cfg.Region != ""
}
