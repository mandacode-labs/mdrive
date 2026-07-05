package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// Drive holds the schema for a drive (multi-tenant storage unit).
// A drive owns its root node (root_node_id) and its storage configuration
// (separated into drive_storage for security and reduced table size).
type Drive struct {
	ent.Schema
}

// Annotations of the Drive (table name: drives).
func (Drive) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "drives"},
	}
}

// Mixin of the Drive.
func (Drive) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

// Fields of the Drive.
func (Drive) Fields() []ent.Field {
	return []ent.Field{
		// Primary identifier (ULID).
		field.String("id").
			StorageKey("id").
			DefaultFunc(func() string {
				return ulid.Make().String()
			}).
			Unique().
			Immutable(),

		// Display name.
		field.String("name").
			MaxLen(64),

		// Description.
		field.String("description").
			Optional().
			Nillable().
			MaxLen(255),

		// Owning user (ULID, references User.id).
		field.String("owner_id"),

		// Root node of this drive (UUID, references Node.id).
		// Set when the drive's root directory is created.
		field.UUID("root_node_id", uuid.UUID{}),

		// Soft-delete timestamp. Null means active.
		field.Time("deleted_at").
			Optional().
			Nillable(),
	}
}

// Indexes of the Drive.
func (Drive) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_id"),
		index.Fields("root_node_id"),
		index.Fields("deleted_at"),
	}
}

// Edges of the Drive.
func (Drive) Edges() []ent.Edge {
	return []ent.Edge{
		// Drive has one storage configuration.
		edge.To("storage", Storage.Type).
			Annotations(entsql.Annotation{
				OnDelete: entsql.Cascade,
			}).
			Unique(),
		edge.To("nodes", Node.Type).
			Annotations(entsql.Annotation{
				OnDelete: entsql.Cascade,
			}),
		edge.From("user", User.Type).
			Ref("drives").
			Field("owner_id").
			Unique().
			Required(),
	}
}
