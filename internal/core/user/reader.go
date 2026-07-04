package user

import "context"

// Reader is the handler-side read surface: lookup by private
// ID, plus the OIDC upsert path used during the auth callback.
type Reader interface {
	UpsertFromOIDC(ctx context.Context, cmd *CreateCommand) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
}

var _ Reader = (*Service)(nil)
