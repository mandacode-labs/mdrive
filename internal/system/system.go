package system

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errors"
)

// Status represents system status.
type Status string

const (
	StatusActive      Status = "active"
	StatusInactive    Status = "inactive"
	StatusMaintenance Status = "maintenance"
)

// System represents a system/node in the cluster.
type System struct {
	id          string
	name        string
	description *string
	status      Status
	createdAt   time.Time
	updatedAt   time.Time
}

// NewSystem creates a new System.
func NewSystem(
	id string,
	name string,
	description *string,
	status Status,
	createdAt time.Time,
	updatedAt time.Time,
) *System {
	return &System{
		id:          id,
		name:        name,
		description: description,
		status:      status,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
	}
}

// Getters
func (s *System) ID() string           { return s.id }
func (s *System) Name() string         { return s.name }
func (s *System) Description() *string { return s.description }
func (s *System) Status() Status       { return s.status }
func (s *System) CreatedAt() time.Time { return s.createdAt }
func (s *System) UpdatedAt() time.Time { return s.updatedAt }

// SystemService defines the interface for system operations.
type SystemService interface {
	Create(ctx context.Context, cmd *CreateCommand) (*System, error)
	GetByID(ctx context.Context, id string) (*System, error)
	GetByName(ctx context.Context, name string) (*System, error)
	Update(ctx context.Context, cmd *UpdateCommand) error
	Delete(ctx context.Context, id string) error
	Find(ctx context.Context, filter Filter) ([]*System, error)
	FindOne(ctx context.Context, filter Filter) (*System, error)
}

// CreateCommand for creating a system (service layer).
type CreateCommand struct {
	Name        string
	Description *string
	Status      Status
}

// UpdateCommand for updating a system (service layer).
type UpdateCommand struct {
	ID          string
	Name        *string
	Description *string
	Status      *Status
}

// Filter for querying systems (service layer).
type Filter struct {
	ID     *string
	Name   *string
	Status *Status
}

// Filter helpers
func ByID(id string) Filter {
	return Filter{ID: &id}
}

func ByName(name string) Filter {
	return Filter{Name: &name}
}

func ByStatus(status Status) Filter {
	return Filter{Status: &status}
}

// toQueryFilter converts service Filter to repository QueryFilter.
func (f Filter) toQueryFilter() *QueryFilter {
	return &QueryFilter{
		ID:     f.ID,
		Name:   f.Name,
		Status: f.Status,
	}
}

type service struct {
	repo        SystemRepository
	inodeSvc    InodeService
	objectSvc   ObjectService
	sysUserSvc  UserService
	sysGroupSvc GroupService
}

// NewService creates a new SystemService.
func NewService(
	repo SystemRepository,
	inodeSvc InodeService,
	objectSvc ObjectService,
	sysUserSvc UserService,
	sysGroupSvc GroupService,
) SystemService {
	return &service{
		repo:        repo,
		inodeSvc:    inodeSvc,
		objectSvc:   objectSvc,
		sysUserSvc:  sysUserSvc,
		sysGroupSvc: sysGroupSvc,
	}
}

func (s *service) Create(ctx context.Context, cmd *CreateCommand) (*System, error) {
	if cmd.Name == "" {
		return nil, errors.BadRequest("name is required")
	}
	if cmd.Status == "" {
		cmd.Status = StatusActive
	}

	// Generate ID for the system
	now := time.Now()
	systemID := uuid.New().String()

	newSystem := NewSystem(
		systemID,
		cmd.Name,
		cmd.Description,
		cmd.Status,
		now,
		now,
	)
	return s.repo.Create(ctx, newSystem)
}

func (s *service) GetByID(ctx context.Context, id string) (*System, error) {
	sys, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if sys == nil {
		return nil, errors.NotFound("system not found")
	}
	return sys, nil
}

func (s *service) GetByName(ctx context.Context, name string) (*System, error) {
	sys, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if sys == nil {
		return nil, errors.NotFound("system not found")
	}
	return sys, nil
}

func (s *service) Update(ctx context.Context, cmd *UpdateCommand) error {
	// Get existing system to handle partial updates
	existing, err := s.repo.GetByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.NotFound("system not found")
	}

	// Build updated system with coalesced values
	updated := NewSystem(
		existing.ID(),
		coalesceString(cmd.Name, existing.Name()),
		coalescePtr(cmd.Description, existing.Description()),
		coalesceStatus(cmd.Status, existing.Status()),
		existing.CreatedAt(),
		existing.UpdatedAt(), // TODO: should be updated in repository
	)
	return s.repo.Update(ctx, updated)
}

// coalesceString returns new if non-empty, otherwise returns existing.
func coalesceString(new *string, existing string) string {
	if new != nil && *new != "" {
		return *new
	}
	return existing
}

// coalescePtr returns new if not nil, otherwise returns existing.
func coalescePtr[T any](new, existing *T) *T {
	if new != nil {
		return new
	}
	return existing
}

// coalesceStatus returns new if not nil, otherwise returns existing.
func coalesceStatus(new *Status, existing Status) Status {
	if new != nil {
		return *new
	}
	return existing
}

func (s *service) Delete(ctx context.Context, id string) error {
	// Verify system exists
	sys, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if sys == nil {
		return errors.NotFound("system not found")
	}

	// Cleanup S3 objects (best-effort)
	_ = s.objectSvc.CleanupStorageBySystemID(ctx, id)

	// Delete inodes
	if err := s.inodeSvc.DeleteBySystemID(ctx, id); err != nil {
		return errors.WrapInternal(err, "failed to delete system inodes")
	}

	// Delete objects from DB
	if err := s.objectSvc.DeleteBySystemID(ctx, id); err != nil {
		return errors.WrapInternal(err, "failed to delete system objects")
	}

	// Delete system groups
	if err := s.sysGroupSvc.DeleteBySystemID(ctx, id); err != nil {
		return errors.WrapInternal(err, "failed to delete system groups")
	}

	// Delete system users
	if err := s.sysUserSvc.DeleteBySystemID(ctx, id); err != nil {
		return errors.WrapInternal(err, "failed to delete system users")
	}

	// Delete system record
	return s.repo.Delete(ctx, id)
}

func (s *service) Find(ctx context.Context, filter Filter) ([]*System, error) {
	return s.repo.Find(ctx, filter.toQueryFilter())
}

func (s *service) FindOne(ctx context.Context, filter Filter) (*System, error) {
	return s.repo.FindOne(ctx, filter.toQueryFilter())
}
