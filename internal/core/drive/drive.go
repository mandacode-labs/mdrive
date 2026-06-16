// Package drive provides the drive domain: multi-tenant storage units.
package drive

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/errors"
	"github.com/mandacode-labs/mdrive/internal/idgen"
)

// Provider represents a storage backend type.
type Provider string

const (
	ProviderS3    Provider = "s3"
	ProviderMinio Provider = "minio"
)

// Drive represents a multi-tenant storage unit.
// A drive owns its root node (Drive.RootNodeID) and storage configuration
// (separated into Storage). Permissions are managed by OpenFGA.
type Drive struct {
	id          string
	publicID    string
	name        string
	description *string
	provider    Provider
	ownerID     string
	rootNodeID  *uuid.UUID
	createdAt   time.Time
	updatedAt   time.Time
}

// NewDrive creates a new Drive.
func NewDrive(
	id string,
	publicID string,
	name string,
	description *string,
	provider Provider,
	ownerID string,
	rootNodeID *uuid.UUID,
	createdAt time.Time,
	updatedAt time.Time,
) *Drive {
	return &Drive{
		id:          id,
		publicID:    publicID,
		name:        name,
		description: description,
		provider:    provider,
		ownerID:     ownerID,
		rootNodeID:  rootNodeID,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
	}
}

// Getters.
func (d *Drive) ID() string           { return d.id }
func (d *Drive) PublicID() string     { return d.publicID }
func (d *Drive) Name() string         { return d.name }
func (d *Drive) Description() *string { return d.description }
func (d *Drive) Provider() Provider   { return d.provider }
func (d *Drive) OwnerID() string      { return d.ownerID }
func (d *Drive) RootNodeID() *uuid.UUID { return d.rootNodeID }
func (d *Drive) CreatedAt() time.Time { return d.createdAt }
func (d *Drive) UpdatedAt() time.Time { return d.updatedAt }

// SetRootNodeID records the root node of this drive.
// Called once during drive creation, after the root directory node is created.
func (d *Drive) SetRootNodeID(id uuid.UUID) {
	d.rootNodeID = &id
	d.updatedAt = time.Now()
}

// StorageConfig holds the S3/MinIO backend configuration (input to CreateCommand).
type StorageConfig struct {
	Bucket       string
	Endpoint     *string
	Region       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
}

// CreateCommand for creating a new drive.
type CreateCommand struct {
	Name        string
	Description *string
	Provider    Provider
	OwnerID     string
	Storage     StorageConfig
}

// UpdateCommand for updating a drive's mutable fields.
type UpdateCommand struct {
	Name        *string
	Description *string
}

// Repository is the data-access contract for drives.
type Repository interface {
	Create(ctx context.Context, d *Drive, s *Storage) error
	GetByID(ctx context.Context, id string) (*Drive, error)
	GetByPublicID(ctx context.Context, publicID string) (*Drive, error)
	GetStorage(ctx context.Context, driveID string) (*Storage, error)
	Update(ctx context.Context, d *Drive) (*Drive, error)
	Delete(ctx context.Context, id string) error
	FindByOwner(ctx context.Context, ownerID string) ([]*Drive, error)
	WithTx(ctx context.Context, fn func(Repository) error) error
}

// Exister checks whether an entity exists. Used to verify owner existence
// without coupling to the user package.
type Exister interface {
	Exists(ctx context.Context, id string) (bool, error)
}

// RootCreator creates the root directory node for a drive.
// Implemented by the node.Service in the application layer (or by a stub
// during testing) to avoid circular dependencies.
type RootCreator interface {
	NewRootDirectory(ctx context.Context) (uuid.UUID, error)
}

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
// Returns the drive (with rootNodeID set) and the root node.
func (s *Service) Create(ctx context.Context, cmd *CreateCommand) (*Drive, uuid.UUID, error) {
	if cmd.Name == "" {
		return nil, uuid.Nil, errors.BadRequest("name is required")
	}
	if cmd.OwnerID == "" {
		return nil, uuid.Nil, errors.BadRequest("owner_id is required")
	}
	if cmd.Storage.Bucket == "" {
		return nil, uuid.Nil, errors.BadRequest("storage.bucket is required")
	}
	if cmd.Storage.Region == "" {
		return nil, uuid.Nil, errors.BadRequest("storage.region is required")
	}
	if cmd.Storage.AccessKey == "" {
		return nil, uuid.Nil, errors.BadRequest("storage.access_key is required")
	}
	if cmd.Storage.SecretKey == "" {
		return nil, uuid.Nil, errors.BadRequest("storage.secret_key is required")
	}

	// Verify owner exists.
	exists, err := s.users.Exists(ctx, cmd.OwnerID)
	if err != nil {
		return nil, uuid.Nil, errors.WrapInternal(err, "check owner")
	}
	if !exists {
		return nil, uuid.Nil, errors.NotFound("owner not found")
	}

	// Create drive + storage atomically.
	id := idgen.Generate()
	provider := cmd.Provider
	if provider == "" {
		provider = ProviderS3
	}
	now := time.Now()
	d := NewDrive(
		id, id, cmd.Name, cmd.Description, provider, cmd.OwnerID, nil, now, now,
	)
	s2 := NewStorage(
		d.id,
		cmd.Storage.Bucket,
		cmd.Storage.Endpoint,
		cmd.Storage.Region,
		cmd.Storage.AccessKey,
		cmd.Storage.SecretKey,
		cmd.Storage.UsePathStyle,
	)

	if err := s.repo.Create(ctx, d, s2); err != nil {
		return nil, uuid.Nil, errors.WrapInternal(err, "create drive")
	}

	// Create the root directory node.
	rootID, err := s.rootCreate.NewRootDirectory(ctx)
	if err != nil {
		// Compensate: try to delete the drive.
		_ = s.repo.Delete(ctx, id)
		return nil, uuid.Nil, errors.WrapInternal(err, "create root node")
	}
	d.SetRootNodeID(rootID)

	// Update the drive to record the root node ID.
	updated, err := s.repo.Update(ctx, d)
	if err != nil {
		return nil, uuid.Nil, errors.WrapInternal(err, "update drive with root")
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
		return nil, errors.NotFound("drive not found")
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
		return nil, errors.NotFound("drive not found")
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
		return nil, errors.NotFound("drive storage not found")
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
		return nil, errors.NotFound("drive not found")
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
		existing.ID(),
		existing.PublicID(),
		name,
		desc,
		existing.Provider(),
		existing.OwnerID(),
		existing.RootNodeID(),
		existing.CreatedAt(),
		time.Now(),
	)
	return s.repo.Update(ctx, updated)
}

// Delete removes a drive. Caller is responsible for:
//  1. Walking the drive's tree to delete all nodes.
//  2. Revoking OpenFGA permissions.
//  3. Cleaning up S3 objects.
//
// This is a hard delete; the S3 data should already be cleaned up.
func (s *Service) Delete(ctx context.Context, id string) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.NotFound("drive not found")
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
