package vfs

import (
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// Drive represents a multi-tenant storage unit.
// A drive owns its root node (Drive.RootNodeID) and storage configuration
// (separated into Storage). Permissions are managed by OpenFGA.
type Drive struct {
	id          ulid.ULID
	name        string
	description *string
	owner       ulid.ULID
	root        *uuid.UUID
	deletedAt   *time.Time
	createdAt   time.Time
	updatedAt   time.Time
}

// Getters
func (d *Drive) ID() ulid.ULID         { return d.id }
func (d *Drive) Name() string          { return d.name }
func (d *Drive) Description() *string  { return d.description }
func (d *Drive) Root() *uuid.UUID      { return d.root }
func (d *Drive) Owner() ulid.ULID      { return d.owner }
func (d *Drive) DeletedAt() *time.Time { return d.deletedAt }
func (d *Drive) CreatedAt() time.Time  { return d.createdAt }
func (d *Drive) UpdatedAt() time.Time  { return d.updatedAt }

func NewDrive(
	id ulid.ULID,
	name string,
	root uuid.UUID,
	owner ulid.ULID,
) *Drive {
	return &Drive{
		id:        id,
		name:      name,
		root:      &root,
		owner:     owner,
		createdAt: time.Now(),
		updatedAt: time.Now(),
		deletedAt: nil,
	}
}

func HydrateDrive(
	id ulid.ULID,
	name string,
	description *string,
	root *uuid.UUID,
	owner ulid.ULID,
	deletedAt *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) *Drive {
	return &Drive{
		id:          id,
		name:        name,
		description: description,
		root:        root,
		owner:       owner,
		deletedAt:   deletedAt,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
	}
}
