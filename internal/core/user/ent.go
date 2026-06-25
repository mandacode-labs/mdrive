package user

import (
	"context"

	"github.com/mandacode-labs/mdrive/ent"
	entuser "github.com/mandacode-labs/mdrive/ent/user"
)

// EntRepository implements domain.Repository using Ent.
type EntRepository struct {
	client *ent.Client
}

// NewRepository creates a new EntRepository.
func NewRepository(client *ent.Client) Repository {
	return &EntRepository{client: client}
}

func (r *EntRepository) Create(ctx context.Context, cmd *CreateCommand) (*User, error) {
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

func (r *EntRepository) GetByID(ctx context.Context, id string) (*User, error) {
	u, err := r.client.User.Query().Where(entuser.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return fromEnt(u), nil
}

func (r *EntRepository) GetByPublicID(ctx context.Context, publicID string) (*User, error) {
	u, err := r.client.User.Query().Where(entuser.PublicIDEQ(publicID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return fromEnt(u), nil
}

func (r *EntRepository) GetByProviderID(ctx context.Context, provider, providerID string) (*User, error) {
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

func (r *EntRepository) Update(ctx context.Context, u *User) (*User, error) {
	updated, err := r.client.User.UpdateOneID(u.ID()).
		SetName(u.Name()).
		SetNillableEmail(u.Email()).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return fromEnt(updated), nil
}

func (r *EntRepository) Delete(ctx context.Context, id string) error {
	return r.client.User.DeleteOneID(id).Exec(ctx)
}

// ExisterAdapter adapts this repository to the user.Exister interface,
// so other packages can verify user existence without depending on user.Service.
type ExisterAdapter struct {
	repo Repository
}

// NewExisterAdapter creates a new ExisterAdapter.
func NewExisterAdapter(repo Repository) Exister {
	return &ExisterAdapter{repo: repo}
}

func (a *ExisterAdapter) Exist(ctx context.Context, id string) (bool, error) {
	u, err := a.repo.GetByID(ctx, id)
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
