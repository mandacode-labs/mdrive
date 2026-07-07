package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"

	"github.com/google/uuid"
)

// Superblock is the filesystem root inode carrier. Linux's
// super_block struct holds the mount info, filesystem type,
// and the root dentry. We mirror that one-to-one: each drive
// has exactly one superblock whose only purpose is to point
// at the root inode.
type Superblock struct {
	ent.Schema
}

// Annotations of the Superblock (table name: superblocks).
func (Superblock) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "superblocks"},
	}
}

// Mixin of the Superblock.
func (Superblock) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

// Fields of the Superblock.
func (Superblock) Fields() []ent.Field {
	return []ent.Field{
		// Primary identifier (UUID).
		field.UUID("id", uuid.UUID{}).
			StorageKey("id").
			Default(uuid.New).
			Unique().
			Immutable(),

		field.String("drive_id").
			Unique().
			Immutable(),

		// Root node inode (UUID, references Node.id). This is
		// what vfs uses to start path resolution.
		field.UUID("root_node_id", uuid.UUID{}).
			Immutable(),
	}
}

// Edges of the Superblock.
func (Superblock) Edges() []ent.Edge {
	return []ent.Edge{
		// Belongs to exactly one drive.
		edge.From("drive", Drive.Type).
			Ref("superblock").
			Field("drive_id").
			Unique().
			Required().
			Immutable(),
		edge.To("nodes", Node.Type).
			Annotations(entsql.Annotation{
				OnDelete: entsql.Cascade,
			}),
	}
}
