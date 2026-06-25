package user

import "context"

// Repository is the data-access contract for users.
type Repository interface {
	Create(ctx context.Context, cmd *CreateCommand) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	GetByPublicID(ctx context.Context, publicID string) (*User, error)
	GetByProviderID(ctx context.Context, provider, providerID string) (*User, error)
	Update(ctx context.Context, u *User) (*User, error)
	Delete(ctx context.Context, id string) error
}

// Exister checks if a user exists. Used by other packages (e.g., drive) to verify
// ownership without depending on the full user.Service.
type Exister interface {
	Exist(ctx context.Context, id string) (bool, error)
}
