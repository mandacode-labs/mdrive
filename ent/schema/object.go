package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

// Object holds the schema definition for the Object entity.
// Tracks actual objects in external object storage (S3, MinIO, etc.).
type Object struct {
	ent.Schema
}

// Annotations of the Object.
func (Object) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "objects"},
	}
}

// Mixin of the Object.
func (Object) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

// Fields of the Object.
func (Object) Fields() []ent.Field {
	return []ent.Field{
		field.String("id"),
		field.Enum("provider").
			Values("s3").
			Default("s3"),
		field.String("bucket"),
		field.String("system_id"),
		// Storage key would be inode ID
		field.String("storage_key"),
		// Status: pending (upload initiated), active (upload complete)
		field.Enum("status").
			Values("pending", "active").
			Default("active"),
		// Checksum for integrity verification (base64-encoded MD5)
		field.String("checksum").
			Optional().
			Nillable(),
		// Idempotency key to prevent duplicate uploads
		field.String("idempotency_key").
			Optional().
			Nillable(),
	}
}

// Indexes of the Object.
func (Object) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("system_id", "provider", "bucket", "storage_key").Unique(),
		index.Fields("system_id"),
		index.Fields("provider", "bucket"),
		// For GC: find pending objects older than threshold
		index.Fields("status", "update_time"),
		// For idempotency lookups
		index.Fields("system_id", "idempotency_key").Unique(),
	}
}

// Edges of the Object.
func (Object) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("system", System.Type).
			Ref("objects").
			Field("system_id").
			Required().
			Unique(),
	}
}
