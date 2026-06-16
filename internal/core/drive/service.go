package drive

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// Service provides domain-level drive operations.
// It wraps Repository with validation and convenience, analogous to
// Linux super_operations (alloc_inode, destroy_inode) but without
// permission checks — those belong to vfs.
type Service struct {
	repo       Repository
	users      Exister
	rootCreate RootCreator
}

// NewService creates a new Service.
func NewService(repo Repository, users Exister, rootCreate RootCreator) *Service {
	return &Service{repo: repo, users: users, rootCreate: rootCreate}
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
	d := NewDrive(id, id, name, desc, ProviderS3, ownerID, nil, now, now)

	s2 := NewStorage(id, cfg.Bucket, cfg.Endpoint, cfg.Region,
		cfg.AccessKey, cfg.SecretKey, cfg.UsePathStyle)

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
		d.Provider(), d.OwnerID(), d.RootNodeID(),
		d.CreatedAt(), time.Now(),
	)
	return s.repo.Update(ctx, updated)
}

// Delete removes a drive. Caller is responsible for cleaning up nodes and S3 objects.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.GetByID(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
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
	return cfg.Bucket != "" && cfg.Region != "" &&
		cfg.AccessKey != "" && cfg.SecretKey != ""
}
