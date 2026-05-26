package repository

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/retrowin-go/ent"
	entsystemgroup "github.com/mandacode-labs/retrowin-go/ent/systemgroup"
	entusersystem "github.com/mandacode-labs/retrowin-go/ent/usersystem"
	"github.com/mandacode-labs/retrowin-go/internal/core/user"
)

// EntSystemUserRepository implements user.SystemUserRepository using Ent.
type EntSystemUserRepository struct {
	client *ent.Client
}

// NewSystemUserRepository creates a new EntSystemUserRepository.
func NewSystemUserRepository(client *ent.Client) user.SystemUserRepository {
	return &EntSystemUserRepository{client: client}
}

func (r *EntSystemUserRepository) Create(ctx context.Context, systemUser *user.SystemUser) (*user.SystemUser, error) {
	entUserSystem, err := r.client.UserSystem.Create().
		SetUserID(systemUser.UserID()).
		SetSystemID(systemUser.SystemID()).
		SetUsername(systemUser.Username()).
		SetUID(systemUser.UID()).
		SetGid(systemUser.GID()).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create system user: %w", err)
	}
	return systemUserFromEnt(entUserSystem), nil
}

func (r *EntSystemUserRepository) GetByID(ctx context.Context, id int) (*user.SystemUser, error) {
	entUserSystem, err := r.client.UserSystem.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get system user: %w", err)
	}
	return systemUserFromEnt(entUserSystem), nil
}

func (r *EntSystemUserRepository) Delete(ctx context.Context, id int) error {
	return r.client.UserSystem.DeleteOneID(id).Exec(ctx)
}

func (r *EntSystemUserRepository) DeleteBySystemID(ctx context.Context, systemID string) error {
	_, err := r.client.UserSystem.Delete().Where(entusersystem.SystemIDEQ(systemID)).Exec(ctx)
	return err
}

func (r *EntSystemUserRepository) Find(ctx context.Context, filter *user.QueryFilter) ([]*user.SystemUser, error) {
	query := r.client.UserSystem.Query()
	query = applySystemUserFilter(query, filter)

	entUserSystems, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find system users: %w", err)
	}
	return systemUserFromEntSlice(entUserSystems), nil
}

func (r *EntSystemUserRepository) FindOne(ctx context.Context, filter *user.QueryFilter) (*user.SystemUser, error) {
	query := r.client.UserSystem.Query()
	query = applySystemUserFilter(query, filter)

	entUserSystem, err := query.Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find system user: %w", err)
	}
	return systemUserFromEnt(entUserSystem), nil
}

// GetNextUID returns the next available UID for the system.
// It finds the max UID/GID in the system and returns max+1, or MinUID if no users exist.
// This ensures the UID doesn't conflict with existing GIDs since user private groups use GID=UID.
func (r *EntSystemUserRepository) GetNextUID(ctx context.Context, systemID string) (int, error) {
	// Find max UID in the system
	maxUID, err := r.client.UserSystem.Query().
		Where(entusersystem.SystemIDEQ(systemID)).
		Aggregate(
			ent.Max(entusersystem.FieldUID),
		).
		Int(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return 0, fmt.Errorf("failed to get max uid: %w", err)
	}

	// Also check max GID to avoid conflicts with private groups
	maxGID, err := r.client.SystemGroup.Query().
		Where(entsystemgroup.SystemIDEQ(systemID)).
		Aggregate(
			ent.Max(entsystemgroup.FieldGid),
		).
		Int(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return 0, fmt.Errorf("failed to get max gid: %w", err)
	}

	// Use the maximum of maxUID, maxGID, or MinUID-1
	start := user.MinUID - 1
	if maxUID > start {
		start = maxUID
	}
	if maxGID > start {
		start = maxGID
	}

	nextUID := start + 1
	if nextUID > user.MaxUID {
		return 0, fmt.Errorf("no available uid (max %d reached)", user.MaxUID)
	}

	// Ensure we start from MinUID
	if nextUID < user.MinUID {
		nextUID = user.MinUID
	}

	return nextUID, nil
}

func applySystemUserFilter(query *ent.UserSystemQuery, filter *user.QueryFilter) *ent.UserSystemQuery {
	if filter == nil {
		return query
	}
	if filter.UserID != nil {
		query = query.Where(entusersystem.UserIDEQ(*filter.UserID))
	}
	if filter.SystemID != nil {
		query = query.Where(entusersystem.SystemIDEQ(*filter.SystemID))
	}
	if filter.Username != nil {
		query = query.Where(entusersystem.UsernameEQ(*filter.Username))
	}
	if filter.UID != nil {
		query = query.Where(entusersystem.UIDEQ(*filter.UID))
	}
	return query
}

func systemUserFromEnt(e *ent.UserSystem) *user.SystemUser {
	return user.NewSystemUser(
		e.ID,
		e.UserID,
		e.SystemID,
		e.Username,
		e.UID,
		e.Gid,
	)
}

func systemUserFromEntSlice(items []*ent.UserSystem) []*user.SystemUser {
	result := make([]*user.SystemUser, len(items))
	for i, e := range items {
		result[i] = systemUserFromEnt(e)
	}
	return result
}
