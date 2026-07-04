package user

import "context"

// Upserter is the slice of user operations the OIDC flow
// needs: create-or-update from claims, lookup by provider ID.
type Upserter interface {
	UpsertFromOIDC(ctx context.Context, cmd *CreateCommand) (*User, error)
	GetByProviderID(ctx context.Context, provider, providerID string) (*User, error)
}

// Reader is the handler-side read surface: lookup by private
// ID, plus the OIDC upsert path used during the auth callback.
type Reader interface {
	UpsertFromOIDC(ctx context.Context, cmd *CreateCommand) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
}

var (
	_ Upserter = (*Service)(nil)
	_ Reader   = (*Service)(nil)
)
