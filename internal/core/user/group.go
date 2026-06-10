package user

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/errors"
)

// SystemGroup represents a group in a system.
type SystemGroup struct {
	id       int
	systemID string
	name     string
	gid      int
}

// NewSystemGroup creates a new SystemGroup.
func NewSystemGroup(
	id int,
	systemID string,
	name string,
	gid int,
) *SystemGroup {
	return &SystemGroup{
		id:       id,
		systemID: systemID,
		name:     name,
		gid:      gid,
	}
}

// Getters
func (g *SystemGroup) ID() int          { return g.id }
func (g *SystemGroup) SystemID() string { return g.systemID }
func (g *SystemGroup) Name() string     { return g.name }
func (g *SystemGroup) GID() int         { return g.gid }

// GroupCreateCommand for creating a system group (service layer).
type GroupCreateCommand struct {
	SystemID string
	Name     string
	GID      int
}

// GroupFilter for querying system groups (service layer).
type GroupFilter = GroupQueryFilter

// Group filter helpers
func GroupBySystemID(systemID string) GroupFilter {
	return GroupFilter{SystemID: &systemID}
}

func GroupByName(name string) GroupFilter {
	return GroupFilter{Name: &name}
}

func GroupByGID(gid int) GroupFilter {
	return GroupFilter{GID: &gid}
}

func GroupBySystemAndName(systemID, name string) GroupFilter {
	return GroupFilter{SystemID: &systemID, Name: &name}
}

func GroupBySystemAndGID(systemID string, gid int) GroupFilter {
	return GroupFilter{SystemID: &systemID, GID: &gid}
}

// GroupService implements group operations.
type GroupService struct {
	repo SystemGroupRepository
}

// NewGroupService creates a new GroupService.
func NewGroupService(repo SystemGroupRepository) *GroupService {
	return &GroupService{repo: repo}
}

func (s *GroupService) Create(ctx context.Context, cmd *GroupCreateCommand) (*SystemGroup, error) {
	if cmd.SystemID == "" {
		return nil, errors.BadRequest("system_id is required")
	}
	if cmd.Name == "" {
		return nil, errors.BadRequest("name is required")
	}

	gid := cmd.GID
	if gid < 0 {
		var err error
		gid, err = s.repo.GetNextGID(ctx, cmd.SystemID)
		if err != nil {
			return nil, fmt.Errorf("failed to assign gid: %w", err)
		}
	}

	newGroup := NewSystemGroup(
		0,
		cmd.SystemID,
		cmd.Name,
		gid,
	)
	return s.repo.Create(ctx, newGroup)
}

func (s *GroupService) GetByID(ctx context.Context, id int) (*SystemGroup, error) {
	g, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, errors.NotFound("system group not found")
	}
	return g, nil
}

func (s *GroupService) GetByGID(ctx context.Context, systemID string, gid int) (*SystemGroup, error) {
	g, err := s.repo.FindOne(ctx, &GroupQueryFilter{
		SystemID: &systemID,
		GID:      &gid,
	})
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, errors.NotFound("system group not found")
	}
	return g, nil
}

func (s *GroupService) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

func (s *GroupService) DeleteBySystemID(ctx context.Context, systemID string) error {
	return s.repo.DeleteBySystemID(ctx, systemID)
}

func (s *GroupService) Find(ctx context.Context, filter GroupFilter) ([]*SystemGroup, error) {
	return s.repo.Find(ctx, &filter)
}

func (s *GroupService) FindOne(ctx context.Context, filter GroupFilter) (*SystemGroup, error) {
	g, err := s.repo.FindOne(ctx, &filter)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, errors.NotFound("system group not found")
	}
	return g, nil
}

func (s *GroupService) AddUserToGroup(ctx context.Context, userSystemID, groupID int) error {
	return s.repo.AddUserToGroup(ctx, userSystemID, groupID)
}

func (s *GroupService) RemoveUserFromGroup(ctx context.Context, userSystemID, groupID int) error {
	return s.repo.RemoveUserFromGroup(ctx, userSystemID, groupID)
}
