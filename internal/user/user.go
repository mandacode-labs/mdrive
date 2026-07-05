package user

import (
	"time"

	"github.com/oklog/ulid/v2"
)

// User is an externally-authenticated user.
// Identity is established by (provider, provider_id) which maps to the OIDC `sub` claim.
type User struct {
	id         string
	publicID   string
	name       string
	email      *string
	provider   string
	providerID string
	createdAt  time.Time
	updatedAt  time.Time
}

// NewUser creates a new User.
func NewUser(
	id string,
	publicID string,
	name string,
	email *string,
	provider string,
	providerID string,
	createdAt time.Time,
	updatedAt time.Time,
) *User {
	return &User{
		id:         id,
		publicID:   publicID,
		name:       name,
		email:      email,
		provider:   provider,
		providerID: providerID,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
}

func (u *User) ID() string           { return u.id }
func (u *User) PublicID() string     { return u.publicID }
func (u *User) Name() string         { return u.name }
func (u *User) Email() *string       { return u.email }
func (u *User) Provider() string     { return u.provider }
func (u *User) ProviderID() string   { return u.providerID }
func (u *User) CreatedAt() time.Time { return u.createdAt }
func (u *User) UpdatedAt() time.Time { return u.updatedAt }

// CreateCommand for upserting a user from OIDC claims.
type CreateCommand struct {
	Name       string
	Email      *string
	Provider   string
	ProviderID string
}

// GenerateID returns a new ULID for use as user ID.
func GenerateID() string {
	return ulid.Make().String()
}
