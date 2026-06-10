package user

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/errors"
	"github.com/mandacode-labs/mdrive/internal/utils"
)

// SystemUser represents a user's membership in a system.
type SystemUser struct {
	id       int
	userID   string
	systemID string
	username string
	uid      int
	gid      int // primary group
}

// NewSystemUser creates a new SystemUser.
func NewSystemUser(
	id int,
	userID string,
	systemID string,
	username string,
	uid int,
	gid int,
) *SystemUser {
	return &SystemUser{
		id:       id,
		userID:   userID,
		systemID: systemID,
		username: username,
		uid:      uid,
		gid:      gid,
	}
}

// Getters
func (su *SystemUser) ID() int          { return su.id }
func (su *SystemUser) UserID() string   { return su.userID }
func (su *SystemUser) SystemID() string { return su.systemID }
func (su *SystemUser) Username() string { return su.username }
func (su *SystemUser) UID() int         { return su.uid }
func (su *SystemUser) GID() int         { return su.gid }

// CreateCommand for creating a system-user (service layer).
type CreateCommand struct {
	UserID   string
	SystemID string
	Username string
	UID      int // Optional: if -1, auto-assigned; 0 is valid for root
}

// Filter for querying system-users (service layer).
type Filter = QueryFilter

// Filter helpers
func ByUserID(userID string) Filter {
	return Filter{UserID: &userID}
}

func BySystemID(systemID string) Filter {
	return Filter{SystemID: &systemID}
}

func ByUsername(username string) Filter {
	return Filter{Username: &username}
}

func ByUserAndSystem(userID, systemID string) Filter {
	return Filter{UserID: &userID, SystemID: &systemID}
}

func BySystemIDAndUID(systemID string, uid int) Filter {
	return Filter{SystemID: &systemID, UID: &uid}
}

const (
	// MinUID is the minimum UID for regular users.
	MinUID = 1000
	// MaxUID is the maximum UID for regular users.
	MaxUID = 65534
)

// Service implements user identity resolution operations.
type Service struct {
	repo      SystemUserRepository
	groupRepo SystemGroupRepository
}

// NewService creates a new Service.
func NewService(repo SystemUserRepository, groupRepo SystemGroupRepository) *Service {
	return &Service{repo: repo, groupRepo: groupRepo}
}

// ResolveUID extracts userID from context and looks up UID for the system.
func (s *Service) ResolveUID(ctx context.Context, systemID string) (int, error) {
	userID, ok := utils.GetUserID(ctx)
	if !ok || userID == "" {
		return 0, errors.Unauthorized("user not authenticated")
	}

	su, err := s.repo.FindOne(ctx, &QueryFilter{
		UserID:   &userID,
		SystemID: &systemID,
	})
	if err != nil {
		return 0, errors.WrapInternal(err, "failed to resolve uid")
	}
	if su == nil {
		return 0, errors.NotFound("user not found in system")
	}
	return su.UID(), nil
}

// ResolveUIDAndGIDs resolves both UID and GIDs for permission checking.
// Returns primary GID + additional group GIDs from group memberships.
func (s *Service) ResolveUIDAndGIDs(ctx context.Context, systemID string) (int, []int, error) {
	userID, ok := utils.GetUserID(ctx)
	if !ok || userID == "" {
		return 0, nil, errors.Unauthorized("user not authenticated")
	}

	su, err := s.repo.FindOne(ctx, &QueryFilter{
		UserID:   &userID,
		SystemID: &systemID,
	})
	if err != nil {
		return 0, nil, errors.WrapInternal(err, "failed to resolve uid")
	}
	if su == nil {
		return 0, nil, errors.NotFound("user not found in system")
	}

	// Get additional group GIDs
	groupGIDs, err := s.groupRepo.FindGIDsByUserSystemID(ctx, su.ID())
	if err != nil {
		return su.UID(), []int{su.GID()}, nil // Fallback to primary group only
	}

	// Combine primary GID with additional GIDs (deduplicate)
	gidSet := make(map[int]bool)
	gidSet[su.GID()] = true
	for _, gid := range groupGIDs {
		gidSet[gid] = true
	}

	gids := make([]int, 0, len(gidSet))
	for gid := range gidSet {
		gids = append(gids, gid)
	}

	return su.UID(), gids, nil
}

func (s *Service) GetByUserAndSystem(ctx context.Context, userID, systemID string) (*SystemUser, error) {
	return s.repo.FindOne(ctx, &QueryFilter{
		UserID:   &userID,
		SystemID: &systemID,
	})
}

// Create creates a new system user with auto-assigned UID/GID and private group.
func (s *Service) Create(ctx context.Context, cmd *CreateCommand) (*SystemUser, error) {
	if cmd.UserID == "" {
		return nil, errors.BadRequest("user_id is required")
	}
	if cmd.SystemID == "" {
		return nil, errors.BadRequest("system_id is required")
	}
	if cmd.Username == "" {
		return nil, errors.BadRequest("username is required")
	}

	// Check if user already exists in this system
	existing, err := s.repo.FindOne(ctx, &QueryFilter{
		UserID:   &cmd.UserID,
		SystemID: &cmd.SystemID,
	})
	if err != nil {
		return nil, errors.WrapInternal(err, "failed to check existing user")
	}
	if existing != nil {
		return nil, errors.Conflict("user already exists in system")
	}

	// Check if username is already taken in this system
	existingUsername, err := s.repo.FindOne(ctx, &QueryFilter{
		SystemID: &cmd.SystemID,
		Username: &cmd.Username,
	})
	if err != nil {
		return nil, errors.WrapInternal(err, "failed to check existing username")
	}
	if existingUsername != nil {
		return nil, errors.Conflict("username already taken in system")
	}

	// Assign UID if not provided (-1 means auto-assign)
	uid := cmd.UID
	if uid < 0 {
		uid, err = s.repo.GetNextUID(ctx, cmd.SystemID)
		if err != nil {
			return nil, errors.WrapInternal(err, "failed to assign uid")
		}
	}

	// Create private group with same GID as UID
	group, err := s.groupRepo.FindOne(ctx, &GroupQueryFilter{
		SystemID: &cmd.SystemID,
		Name:     &cmd.Username,
	})
	if err != nil {
		return nil, errors.WrapInternal(err, "failed to check existing group")
	}

	if group == nil {
		// Create private group
		newGroup := NewSystemGroup(
			0, // ID is auto-generated by database
			cmd.SystemID,
			cmd.Username,
			uid, // GID = UID
		)
		group, err = s.groupRepo.Create(ctx, newGroup)
		if err != nil {
			return nil, errors.WrapInternal(err, "failed to create private group")
		}
	}

	// Create system user with GID = UID
	newUser := NewSystemUser(
		0, // ID is auto-generated by database
		cmd.UserID,
		cmd.SystemID,
		cmd.Username,
		uid,
		group.GID(), // Use group's GID (same as UID)
	)
	return s.repo.Create(ctx, newUser)
}

func (s *Service) GetByID(ctx context.Context, id int) (*SystemUser, error) {
	su, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if su == nil {
		return nil, errors.NotFound("system user not found")
	}
	return su, nil
}

func (s *Service) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) DeleteBySystemID(ctx context.Context, systemID string) error {
	return s.repo.DeleteBySystemID(ctx, systemID)
}
func (s *Service) Find(ctx context.Context, filter Filter) ([]*SystemUser, error) {
	return s.repo.Find(ctx, &filter)
}

func (s *Service) FindOne(ctx context.Context, filter Filter) (*SystemUser, error) {
	su, err := s.repo.FindOne(ctx, &filter)
	if err != nil {
		return nil, err
	}
	if su == nil {
		return nil, errors.NotFound("system user not found")
	}
	return su, nil
}
