package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"

	"github.com/google/uuid"
)

// Node holds the POSIX-style inode schema.
// One row per node; filename and parent are stored in the parent directory's
// inline DirContent (not in this row), matching Linux where i_parent/i_name
// are absent from the inode structure.
//
// Drive ownership is implicit: a node belongs to the drive whose root
// ancestor chain it can be reached from. The drive's root_node_id acts
// as the entry point, analogous to superblock.s_root in Linux.
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
			Unique().
			Immutable(),

		// Node type (file, directory, symlink, object, device).
		field.Enum("type").
			Values("file", "directory", "symlink", "object", "device").
			Default("file"),

		// Size in bytes. For objects, this is the size of the externally-stored data.
		field.Int64("size").
			Default(0),

		// Hard-link count.
		field.Uint32("nlink").
			Default(1),

		// Inline content (JSON-serialized). Max 4 KiB; nil for large files / objects.
		field.Bytes("content").
			Optional().
			Nillable().
			MaxLen(4096),

		// POSIX timestamps.
		field.Time("atime"),
		field.Time("mtime"),
		field.Time("ctime"),
		field.Time("crtime"),

		// Bitmask of node-level flags (ext2-style i_flags).
		field.Uint32("flags").
			Default(0),

		// Generation identifier (ULID) for optimistic concurrency.
		field.String("revision").
			MaxLen(26),
	}
}

// Indexes of the Node.
func (Node) Indexes() []ent.Index {
	return nil
}
