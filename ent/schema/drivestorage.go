package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// DriveStorage holds the S3/MinIO backend configuration for a drive.
// Kept as a separate table so that drive metadata stays small and
// storage changes don't require touching the drives table.
type DriveStorage struct {
	ent.Schema
}

// Annotations of the DriveStorage (table name: drive_storage).
func (DriveStorage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "drive_storage"},
	}
}

// Fields of the DriveStorage.
func (DriveStorage) Fields() []ent.Field {
	return []ent.Field{
		// Drive ID (PK + FK to drives.id).
		field.String("drive_id").
			MaxLen(32).
			Unique(),

		// Bucket name.
		field.String("bucket").
			MaxLen(128),

		// S3 endpoint (optional for AWS S3).
		field.String("endpoint").
			Optional().
			Nillable().
			MaxLen(255),

		// AWS region.
		field.String("region").
			MaxLen(32),

		// Access key.
		field.String("access_key").
			MaxLen(255),

		// Secret key (sensitive, never log).
		field.String("secret_key").
			MaxLen(255),

		// Use path-style addressing (required for MinIO).
		field.Bool("use_path_style").
			Default(false),

		// Wrapped per-drive data encryption key (Phase 3a). The DEK is
		// a random 32-byte key encrypted with the master key (KEK)
		// and stored as base64(nonce(12) || aesgcm-ciphertext || tag(16)).
		// Nullable for backfill of pre-existing rows; newly created
		// drives always populate this column.
		field.String("wrapped_dek").
			Optional().
			Nillable().
			MaxLen(1024),
	}
}

// Edges of the DriveStorage.
func (DriveStorage) Edges() []ent.Edge {
	return []ent.Edge{
		// Storage belongs to one drive.
		edge.From("drive", Drive.Type).
			Ref("storage").
			Field("drive_id").
			Required().
			Unique(),
	}
}
