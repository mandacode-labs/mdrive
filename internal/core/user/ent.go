package user

import (
	"context"

	"github.com/mandacode-labs/mdrive/ent"
	entuser "github.com/mandacode-labs/mdrive/ent/user"
)

// entRepository implements domain.Repository using Ent.
type entRepository struct {
	client *ent.Client
}

// NewRepository creates a new entRepository.
func NewRepository(client *ent.Client) Repository {
	return &entRepository{client: client}
}

func (r *entRepository) Create(ctx context.Context, cmd *CreateCommand) (*User, error) {
	id := GenerateID()
	u, err := r.client.User.Create().
		SetID(id).
		SetPublicID(id).
		SetName(cmd.Name).
		SetNillableEmail(cmd.Email).
		SetProvider(cmd.Provider).
		SetProviderID(cmd.ProviderID).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return fromEnt(u), nil
}

func (r *entRepository) GetByID(ctx context.Context, id string) (*User, error) {
	u, err := r.client.User.Query().Where(entuser.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return fromEnt(u), nil
}

func (r *entRepository) GetByPublicID(ctx context.Context, publicID string) (*User, error) {
	u, err := r.client.User.Query().Where(entuser.PublicIDEQ(publicID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return fromEnt(u), nil
}

func (r *entRepository) GetByProviderID(ctx context.Context, provider, providerID string) (*User, error) {
	u, err := r.client.User.Query().
		Where(entuser.Provider(provider), entuser.ProviderID(providerID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return fromEnt(u), nil
}

func (r *entRepository) Update(ctx context.Context, u *User) (*User, error) {
	updated, err := r.client.User.UpdateOneID(u.ID()).
		SetName(u.Name()).
		SetNillableEmail(u.Email()).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return fromEnt(updated), nil
}

func (r *entRepository) Delete(ctx context.Context, id string) error {
	return r.client.User.DeleteOneID(id).Exec(ctx)
}

// Exist reports whether a user with the given ID exists.
func (r *entRepository) Exist(ctx context.Context, id string) (bool, error) {
	u, err := r.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	return u != nil, nil
}

func fromEnt(e *ent.User) *User {
	if e == nil {
		return nil
	}
	return NewUser(
		e.ID,
		e.PublicID,
		e.Name,
		e.Email,
		e.Provider,
		e.ProviderID,
		e.CreateTime,
		e.UpdateTime,
	)
}
