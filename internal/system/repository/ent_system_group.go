package repository

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/retrowin-go/ent"
	entsystemgroup "github.com/mandacode-labs/retrowin-go/ent/systemgroup"
	entusergroup "github.com/mandacode-labs/retrowin-go/ent/usergroup"
	entusersystem "github.com/mandacode-labs/retrowin-go/ent/usersystem"
	"github.com/mandacode-labs/retrowin-go/internal/core/user"
)

// EntSystemGroupRepository implements user.SystemGroupRepository using Ent.
type EntSystemGroupRepository struct {
	client *ent.Client
}

// NewSystemGroupRepository creates a new EntSystemGroupRepository.
func NewSystemGroupRepository(client *ent.Client) user.SystemGroupRepository {
	return &EntSystemGroupRepository{client: client}
}

func (r *EntSystemGroupRepository) Create(ctx context.Context, group *user.SystemGroup) (*user.SystemGroup, error) {
	entGroup, err := r.client.SystemGroup.Create().
		SetSystemID(group.SystemID()).
		SetName(group.Name()).
		SetGid(group.GID()).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create system group: %w", err)
	}
	return systemGroupFromEnt(entGroup), nil
}

func (r *EntSystemGroupRepository) GetByID(ctx context.Context, id int) (*user.SystemGroup, error) {
	entGroup, err := r.client.SystemGroup.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get system group: %w", err)
	}
	return systemGroupFromEnt(entGroup), nil
}

func (r *EntSystemGroupRepository) Delete(ctx context.Context, id int) error {
	return r.client.SystemGroup.DeleteOneID(id).Exec(ctx)
}

func (r *EntSystemGroupRepository) DeleteBySystemID(ctx context.Context, systemID string) error {
	_, err := r.client.SystemGroup.Delete().Where(entsystemgroup.SystemIDEQ(systemID)).Exec(ctx)
	return err
}

func (r *EntSystemGroupRepository) Find(ctx context.Context, filter *user.GroupQueryFilter) ([]*user.SystemGroup, error) {
	query := r.client.SystemGroup.Query()
	query = applySystemGroupFilter(query, filter)

	entGroups, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find system groups: %w", err)
	}
	return systemGroupFromEntSlice(entGroups), nil
}

func (r *EntSystemGroupRepository) FindOne(ctx context.Context, filter *user.GroupQueryFilter) (*user.SystemGroup, error) {
	query := r.client.SystemGroup.Query()
	query = applySystemGroupFilter(query, filter)

	entGroup, err := query.Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find system group: %w", err)
	}
	return systemGroupFromEnt(entGroup), nil
}

func (r *EntSystemGroupRepository) AddUserToGroup(ctx context.Context, userSystemID, groupID int) error {
	_, err := r.client.UserGroup.Create().
		SetUserSystemID(userSystemID).
		SetSystemGroupID(groupID).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to add user to group: %w", err)
	}
	return nil
}

func (r *EntSystemGroupRepository) RemoveUserFromGroup(ctx context.Context, userSystemID, groupID int) error {
	_, err := r.client.UserGroup.Delete().
		Where(
			entusergroup.UserSystemIDEQ(userSystemID),
			entusergroup.SystemGroupIDEQ(groupID),
		).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove user from group: %w", err)
	}
	return nil
}

func (r *EntSystemGroupRepository) FindGIDsByUserSystemID(ctx context.Context, userSystemID int) ([]int, error) {
	groups, err := r.client.SystemGroup.Query().
		Where(
			entsystemgroup.HasUsersWith(
				entusersystem.IDEQ(userSystemID),
			),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find user groups: %w", err)
	}

	gids := make([]int, len(groups))
	for i, g := range groups {
		gids[i] = g.Gid
	}
	return gids, nil
}

// GetNextGID returns the next available GID for the system.
// It finds the max GID in the system and returns max+1, or MinGID if no groups exist.
func (r *EntSystemGroupRepository) GetNextGID(ctx context.Context, systemID string) (int, error) {
	// Find max GID in the system
	maxGID, err := r.client.SystemGroup.Query().
		Where(entsystemgroup.SystemIDEQ(systemID)).
		Aggregate(
			ent.Max(entsystemgroup.FieldGid),
		).
		Int(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return user.MinUID, nil // No groups, start from MinUID (same as MinGID)
		}
		return 0, fmt.Errorf("failed to get max gid: %w", err)
	}

	nextGID := maxGID + 1
	if nextGID > user.MaxUID {
		return 0, fmt.Errorf("no available gid (max %d reached)", user.MaxUID)
	}

	// Ensure we start from MinUID
	if nextGID < user.MinUID {
		nextGID = user.MinUID
	}

	return nextGID, nil
}

func applySystemGroupFilter(query *ent.SystemGroupQuery, filter *user.GroupQueryFilter) *ent.SystemGroupQuery {
	if filter == nil {
		return query
	}
	if filter.SystemID != nil {
		query = query.Where(entsystemgroup.SystemIDEQ(*filter.SystemID))
	}
	if filter.Name != nil {
		query = query.Where(entsystemgroup.NameEQ(*filter.Name))
	}
	if filter.GID != nil {
		query = query.Where(entsystemgroup.GidEQ(*filter.GID))
	}
	return query
}

func systemGroupFromEnt(e *ent.SystemGroup) *user.SystemGroup {
	return user.NewSystemGroup(
		e.ID,
		e.SystemID,
		e.Name,
		e.Gid,
	)
}

func systemGroupFromEntSlice(items []*ent.SystemGroup) []*user.SystemGroup {
	result := make([]*user.SystemGroup, len(items))
	for i, e := range items {
		result[i] = systemGroupFromEnt(e)
	}
	return result
}
