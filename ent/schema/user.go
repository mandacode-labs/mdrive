package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

// User holds the schema definition for an externally-authenticated user.
// Identity is (provider, provider_id) which maps to the OIDC `sub` claim.
type User struct {
	ent.Schema
}

// Annotations of the User (table name: users).
func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "users"},
	}
}

// Mixin of the User.
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		// Primary identifier (ULID, 26 chars).
		field.String("id").
			MaxLen(32).
			Unique().
			Immutable(),

		// Public ID (also ULID) for external exposure.
		field.String("public_id").
			MaxLen(32).
			Unique(),

		// Display name (from OIDC).
		field.String("name").
			MaxLen(255),

		// Email (from OIDC, optional).
		field.String("email").
			Optional().
			Nillable().
			MaxLen(255),

		// OIDC provider (e.g., "zitadel", "keycloak", "google").
		field.String("provider").
			MaxLen(32),

		// OIDC provider's user ID (the `sub` claim).
		field.String("provider_id").
			MaxLen(255),
	}
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		// For OIDC upsert lookup.
		index.Fields("provider", "provider_id").Unique(),
	}
}
