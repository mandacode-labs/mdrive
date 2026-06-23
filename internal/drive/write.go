package drive

import (
	"context"

	"github.com/google/uuid"

	coredrive "github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Create creates a drive with storage config and root node, then
// grants owner permission in OpenFGA (best-effort: a grant failure
// is swallowed because the drive already exists; a future orphan
// scan can re-grant).
func (s *Service) Create(ctx context.Context, actorID string, name, description string, cfg coredrive.StorageConfig) (*coredrive.Drive, uuid.UUID, error) {
	d, rootID, err := s.Drive.Create(ctx, name, &description, actorID, cfg)
	if err != nil {
		return nil, uuid.Nil, err
	}
	if s.Perm != nil {
		_ = s.Perm.Grant(ctx, actorID, permission.RelationOwner, permission.ObjectTypeDrive, d.ID())
	}
	return d, rootID, nil
}

// Update updates drive fields. Requires edit.
func (s *Service) Update(ctx context.Context, actorID, id string, name, description *string) (*coredrive.Drive, error) {
	if err := s.checkAccess(ctx, actorID, permission.PermissionEdit, id); err != nil {
		return nil, err
	}
	return s.Drive.Update(ctx, id, name, description)
}

// Delete soft-deletes a drive. Requires delete.
func (s *Service) Delete(ctx context.Context, actorID, id string) error {
	if err := s.checkAccess(ctx, actorID, permission.PermissionDelete, id); err != nil {
		return err
	}
	return s.Drive.Delete(ctx, id)
}

// Restore reactivates a soft-deleted drive. Requires manage.
func (s *Service) Restore(ctx context.Context, actorID, id string) (*coredrive.Drive, error) {
	if err := s.checkAccess(ctx, actorID, permission.PermissionManage, id); err != nil {
		return nil, err
	}
	return s.Drive.Restore(ctx, id)
}
