package drive

import (
	"time"

	"github.com/oklog/ulid/v2"
)

// Drive is the multi-tenant storage unit metadata. The
// filesystem root inode lives in superblock.Superblock; this
// type carries only the lifecycle fields.
type Drive struct {
	id          ulid.ULID
	name        string
	description *string
	owner       ulid.ULID
	deletedAt   *time.Time
	createdAt   time.Time
	updatedAt   time.Time
}

func (d *Drive) ID() ulid.ULID         { return d.id }
func (d *Drive) Name() string          { return d.name }
func (d *Drive) Description() *string  { return d.description }
func (d *Drive) Owner() ulid.ULID      { return d.owner }
func (d *Drive) DeletedAt() *time.Time { return d.deletedAt }
func (d *Drive) CreatedAt() time.Time  { return d.createdAt }
func (d *Drive) UpdatedAt() time.Time  { return d.updatedAt }

// SetDescription records the description pointer. Caller may
// pass nil to clear.
func (d *Drive) SetDescription(desc *string) {
	d.description = desc
}

func New(id ulid.ULID, name string, owner ulid.ULID) *Drive {
	now := time.Now()
	return &Drive{
		id:        id,
		name:      name,
		owner:     owner,
		createdAt: now,
		updatedAt: now,
		deletedAt: nil,
	}
}

func Hydrate(
	id ulid.ULID,
	name string,
	description *string,
	owner ulid.ULID,
	deletedAt *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) *Drive {
	return &Drive{
		id:          id,
		name:        name,
		description: description,
		owner:       owner,
		deletedAt:   deletedAt,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
	}
}