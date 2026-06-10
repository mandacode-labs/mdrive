package repository

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/ent"
	entuser "github.com/mandacode-labs/mdrive/ent/user"
	domain "github.com/mandacode-labs/mdrive/internal/user"
)

// EntRepository implements domain.UserRepository using Ent.
type EntRepository struct {
	client *ent.Client
}

// NewRepository creates a new EntRepository.
func NewRepository(client *ent.Client) domain.UserRepository {
	return &EntRepository{client: client}
}

func (r *EntRepository) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	entUser, err := r.client.User.
		Create().
		SetID(user.ID()).
		SetUsername(user.Username()).
		SetProvider(user.Provider()).
		SetProviderID(user.ProviderID()).
		SetJoinDate(user.JoinDate()).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return fromEnt(entUser), nil
}

func (r *EntRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	entUser, err := r.client.User.
		Query().
		Where(entuser.ID(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return fromEnt(entUser), nil
}

func (r *EntRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	entUser, err := r.client.User.
		Query().
		Where(entuser.Username(username)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return fromEnt(entUser), nil
}

func (r *EntRepository) GetByProvider(ctx context.Context, provider, providerID string) (*domain.User, error) {
	entUser, err := r.client.User.
		Query().
		Where(
			entuser.Provider(provider),
			entuser.ProviderID(providerID),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return fromEnt(entUser), nil
}

func (r *EntRepository) Delete(ctx context.Context, id string) error {
	return r.client.User.DeleteOneID(id).Exec(ctx)
}

func (r *EntRepository) ExistsByProvider(ctx context.Context, provider, providerID string) (bool, error) {
	return r.client.User.
		Query().
		Where(
			entuser.Provider(provider),
			entuser.ProviderID(providerID),
		).
		Exist(ctx)
}

func fromEnt(e *ent.User) *domain.User {
	return domain.NewUser(
		e.ID,
		e.Username,
		e.Provider,
		e.ProviderID,
		e.JoinDate,
		e.CreateTime,
		e.UpdateTime,
	)
}
