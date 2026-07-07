package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Storage holds the schema definition for the Storage entity.
type Storage struct {
	ent.Schema
}

// Fields of the Storage.
func (Storage) Fields() []ent.Field {
	return []ent.Field{
		field.String("drive_id").
			Immutable().
			Unique(),

		// Storage provider.
		field.Enum("provider").
			Values("s3", "minio").
			Default("s3"),

		// Bucket name.
		field.String("bucket").
			Optional().
			Nillable(),

		// S3 endpoint (optional for AWS S3).
		field.String("endpoint"),

		// AWS region.
		field.String("region").
			Optional().
			Nillable(),

		field.Bool("use_path_style").
			Optional().
			Nillable(),

		// Access key.
		field.String("access_key").
			Sensitive().
			Optional().
			Nillable(),

		// Secret key (sensitive, never log).
		field.String("encrypted_secret_key").
			Sensitive().
			Optional().
			Nillable(),
	}
}

// Edges of the Storage.
func (Storage) Edges() []ent.Edge {
	return []ent.Edge{
		// Storage belongs to one drive.
		edge.From("drive", Drive.Type).
			Ref("storage").
			Field("drive_id").
			Immutable().
			Required().
			Unique(),
	}
}
