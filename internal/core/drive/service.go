package drive

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/core/user"
)

// Service implements drive operations.
type Service struct {
	repo       Repository
	users      user.Exister
	rootCreate RootCreator
}

// NewService creates a new Service.
func NewService(repo Repository, users user.Exister, rootCreate RootCreator) *Service {
	return &Service{repo: repo, users: users, rootCreate: rootCreate}
}

// Create creates a new drive with storage configuration and its root directory node.
// Returns the drive (with rootNodeID set) and the root node's UUID.
func (s *Service) Create(ctx context.Context, cmd *CreateCommand) (*Drive, uuid.UUID, error) {
	if cmd.Name == "" {
		return nil, uuid.Nil, ErrInvalidName
	}
	if cmd.OwnerID == "" {
		return nil, uuid.Nil, fmt.Errorf("drive: owner_id is required")
	}
	if cmd.Storage.Bucket == "" {
		return nil, uuid.Nil, ErrInvalidBucket
	}
	if cmd.Storage.Region == "" {
		return nil, uuid.Nil, ErrInvalidRegion
	}
	if cmd.Storage.AccessKey == "" || cmd.Storage.SecretKey == "" {
		return nil, uuid.Nil, ErrInvalidCredentials
	}

	exists, err := s.users.Exists(ctx, cmd.OwnerID)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("check owner: %w", err)
	}
	if !exists {
		return nil, uuid.Nil, fmt.Errorf("drive: owner not found")
	}

	id := generateID()
	provider := cmd.Provider
	if provider == "" {
		provider = ProviderS3
	}
	now := time.Now()
	d := NewDrive(id, id, cmd.Name, cmd.Description, provider, cmd.OwnerID, nil, now, now)
	s2 := NewStorage(d.id, cmd.Storage.Bucket, cmd.Storage.Endpoint, cmd.Storage.Region,
		cmd.Storage.AccessKey, cmd.Storage.SecretKey, cmd.Storage.UsePathStyle)

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

// Update updates a drive's mutable fields.
func (s *Service) Update(ctx context.Context, id string, cmd *UpdateCommand) (*Drive, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrNotFound
	}
	name := existing.Name()
	if cmd.Name != nil {
		name = *cmd.Name
	}
	desc := existing.Description()
	if cmd.Description != nil {
		desc = cmd.Description
	}
	updated := NewDrive(
		existing.ID(), existing.PublicID(), name, desc,
		existing.Provider(), existing.OwnerID(), existing.RootNodeID(),
		existing.CreatedAt(), time.Now(),
	)
	return s.repo.Update(ctx, updated)
}

// Delete removes a drive. Caller is responsible for:
//  1. Walking the drive's tree to delete all nodes.
//  2. Revoking OpenFGA permissions.
//  3. Cleaning up S3 objects.
func (s *Service) Delete(ctx context.Context, id string) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrNotFound
	}
	return s.repo.Delete(ctx, id)
}

// ListByOwner returns all drives owned by the given user.
func (s *Service) ListByOwner(ctx context.Context, ownerID string) ([]*Drive, error) {
	return s.repo.FindByOwner(ctx, ownerID)
}

// WithTx executes fn within a transaction.
func (s *Service) WithTx(ctx context.Context, fn func(*Service) error) error {
	return s.repo.WithTx(ctx, func(txRepo Repository) error {
		return fn(&Service{repo: txRepo, users: s.users, rootCreate: s.rootCreate})
	})
}

// generateID returns a new ULID for use as drive ID.
func generateID() string {
	return ulid.Make().String()
}
