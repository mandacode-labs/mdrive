package drive

import (
	"time"

	"github.com/google/uuid"
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
	deletedAt   *time.Time
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
	deletedAt *time.Time,
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
		deletedAt:   deletedAt,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
	}
}

func (d *Drive) ID() string             { return d.id }
func (d *Drive) PublicID() string       { return d.publicID }
func (d *Drive) Name() string           { return d.name }
func (d *Drive) Description() *string   { return d.description }
func (d *Drive) Provider() Provider     { return d.provider }
func (d *Drive) OwnerID() string        { return d.ownerID }
func (d *Drive) RootNodeID() *uuid.UUID { return d.rootNodeID }
func (d *Drive) DeletedAt() *time.Time  { return d.deletedAt }
func (d *Drive) CreatedAt() time.Time   { return d.createdAt }
func (d *Drive) UpdatedAt() time.Time   { return d.updatedAt }

// SetRootNodeID records the root node of this drive.
// Called once during drive creation, after the root directory node is created.
func (d *Drive) SetRootNodeID(id uuid.UUID) {
	d.rootNodeID = &id
	d.updatedAt = time.Now()
}

// SetDeletedAt records the soft-delete timestamp.
func (d *Drive) SetDeletedAt(t *time.Time) {
	d.deletedAt = t
	d.updatedAt = time.Now()
}
