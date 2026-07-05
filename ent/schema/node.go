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

// Node holds the schema definition for the Node entity.
type Node struct {
	ent.Schema
}

// Annotations of the Node (table name: nodes).
func (Node) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "nodes"},
	}
}

// Mixin of the Node.
func (Node) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

// Fields of the Node.
func (Node) Fields() []ent.Field {
	return []ent.Field{
		// Primary identifier (UUID).
		field.UUID("id", uuid.UUID{}).
			StorageKey("id").
			Default(uuid.New).
			Unique().
			Immutable(),

		// Drive ID
		field.String("drive_id"),

		// Node kind (file, directory, symlink, object, mount).
		field.Enum("kind").
			Values("file", "directory", "symlink", "object", "mount").
			Default("file"),

		// Size in bytes. For objects, this is the size of the externally-stored data.
		field.Int64("size").
			Default(0),

		// Hard-link count (POSIX: fresh inode has nlink=0).
		field.Uint32("nlink").
			Default(0),

		// Inline Data (JSON-serialized). Max 4 KiB; nil for large files / objects.
		field.Bytes("data").
			Optional().
			MaxLen(4096),

		// POSIX timestamps.
		field.Time("atime"),
		field.Time("mtime").
			StorageKey("updated_at"),
		field.Time("ctime"),
		field.Time("crtime").
			StorageKey("created_at"),

		// Bitmask of node-level flags (ext2-style i_flags).
		field.Uint32("flags").
			Default(0),

		// Generation identifier (ULID) for optimistic concurrency.
		field.String("revision").
			MaxLen(26),
	}
}

func (Node) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("drive", Drive.Type).
			Ref("nodes").
			Field("drive_id").
			Unique().
			Required(),
	}
}

// Indexes of the Node.
func (Node) Indexes() []ent.Index {
	return nil
}
