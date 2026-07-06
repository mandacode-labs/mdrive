package drive

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"
)

// Service is the public surface for drive lifecycle. Handlers
// call into this; vfs does not depend on Service (it uses
// superblock.Operation for root access).
//
// Renamed from Operation to Service — Service is the conventional
// Go name for a domain layer that orchestrates the data-access
// layer (Repository) for callers.
type Service interface {
	Create(ctx context.Context, ownerID string, name string, description string, storage *Storage) (*Drive, error)
	Get(ctx context.Context, driveID string) (*Drive, error)
	GetStorage(ctx context.Context, driveID string) (*Storage, error)
	Update(ctx context.Context, driveID string, name string, description string) (*Drive, error)
	SoftDelete(ctx context.Context, driveID string) error
	Restore(ctx context.Context, driveID string) (*Drive, error)
	Purge(ctx context.Context, driveID string) error
	ListByOwner(ctx context.Context, ownerID string) ([]*Drive, error)
	ListDeleted(ctx context.Context, before time.Time, limit int) ([]*Drive, error)
}

type service struct {
	repo Repository
}

// NewService wires the canonical impl.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

var _ Service = (*service)(nil)

func (s *service) Create(ctx context.Context, ownerID string, name string, description string, storage *Storage) (*Drive, error) {
	if name == "" {
		return nil, errorxKindInvalidArgument("drive: name is required")
	}
	if ownerID == "" {
		return nil, errorxKindInvalidArgument("drive: owner_id is required")
	}
	if storage == nil {
		return nil, errorxKindInvalidArgument("drive: storage is required")
	}
	ownerULID, err := ulid.Parse(ownerID)
	if err != nil {
		return nil, errorxKindInvalidArgument("drive: invalid owner id")
	}
	id := ulid.Make()

	var descPtr *string
	if description != "" {
		descPtr = &description
	}

	d := New(id, name, ownerULID)
	d.SetDescription(descPtr)

	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}

	storageRow := NewStorage(
		id.String(),
		storage.Bucket(),
		storage.Endpoint(),
		storage.Region(),
		storage.AccessKey(),
		storage.SecretKey(),
		storage.UsePathStyle(),
	)
	if err := s.repo.CreateStorage(ctx, storageRow); err != nil {
		return nil, err
	}

	created, err := s.repo.UpdateFields(ctx, id, name, descPtr)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *service) Get(ctx context.Context, driveID string) (*Drive, error) {
	id, err := parseDriveID(driveID)
	if err != nil {
		return nil, err
	}
	return s.repo.Read(ctx, id)
}

func (s *service) GetStorage(ctx context.Context, driveID string) (*Storage, error) {
	return s.repo.ReadStorage(ctx, driveID)
}

func (s *service) Update(ctx context.Context, driveID string, name string, description string) (*Drive, error) {
	id, err := parseDriveID(driveID)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.Read(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errorxKindNotFound("drive: not found")
	}
	newName := existing.Name()
	if name != "" {
		newName = name
	}
	var descPtr *string
	if description != "" {
		descPtr = &description
	}
	return s.repo.UpdateFields(ctx, id, newName, descPtr)
}

func (s *service) SoftDelete(ctx context.Context, driveID string) error {
	id, err := parseDriveID(driveID)
	if err != nil {
		return err
	}
	if _, err := s.repo.Read(ctx, id); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, id, time.Now())
}

func (s *service) Restore(ctx context.Context, driveID string) (*Drive, error) {
	id, err := parseDriveID(driveID)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.Read(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.DeletedAt() == nil {
		return nil, errorxKindFailedPrecondition("drive: not deleted")
	}
	if err := s.repo.Restore(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.Read(ctx, id)
}

func (s *service) Purge(ctx context.Context, driveID string) error {
	id, err := parseDriveID(driveID)
	if err != nil {
		return err
	}
	return s.repo.Destroy(ctx, id)
}

func (s *service) ListByOwner(ctx context.Context, ownerID string) ([]*Drive, error) {
	if ownerID == "" {
		return nil, errorxKindInvalidArgument("drive: owner_id is required")
	}
	return s.repo.ListByOwner(ctx, ownerID)
}

func (s *service) ListDeleted(ctx context.Context, before time.Time, limit int) ([]*Drive, error) {
	if limit <= 0 {
		limit = 1000
	}
	return s.repo.ListDeleted(ctx, before, limit)
}