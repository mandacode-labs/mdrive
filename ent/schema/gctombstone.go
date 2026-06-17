package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

// GCTombstone records S3 objects that need to be deleted after their
// corresponding node has been removed from the database.
//
// The node delete + tombstone insert happen in a single database
// transaction, so orphans are impossible. The GC worker periodically
// drains this table and issues batch S3 DeleteObjects calls.
type GCTombstone struct {
	ent.Schema
}

func (GCTombstone) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "gc_tombstones"},
	}
}

func (GCTombstone) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

func (GCTombstone) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").
			StorageKey("id").
			Unique().
			Immutable(),

		field.String("bucket").
			MaxLen(128).
			Immutable(),

		field.String("key").
			MaxLen(512).
			Immutable(),

		field.Int("retries").
			Default(0),
	}
}
