// Package drive hosts the vfs-level drive service: it composes
// the core drive package (CRUD data access) with permission checks
// (view, edit, delete, manage) and OpenFGA grants on create, so
// the HTTP handler doesn't have to.
//
// The package is at top level (not under vfs) because drive CRUD
// has nothing to do with the node tree or path resolution — it is
// a sibling concern that the vfs layer happens to share a
// permission system with. vfs is for node management; this
// package is for drive management.
package drive

import (
	"context"
	"time"

	"github.com/google/uuid"

	coredrive "github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Client is the data-access contract the drive service needs. The
// core drive.Service satisfies it; tests may pass fakes.
type Client interface {
	Create(ctx context.Context, name string, desc *string, ownerID string, cfg coredrive.StorageConfig) (*coredrive.Drive, uuid.UUID, error)
	GetByID(ctx context.Context, id string) (*coredrive.Drive, error)
	GetByPublicID(ctx context.Context, pubID string) (*coredrive.Drive, error)
	GetStorage(ctx context.Context, driveID string) (*coredrive.Storage, error)
	Update(ctx context.Context, id string, name, description *string) (*coredrive.Drive, error)
	Delete(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) (*coredrive.Drive, error)
	ListDeleted(ctx context.Context, before time.Time, limit int) ([]*coredrive.Drive, error)
	ListByOwner(ctx context.Context, ownerID string) ([]*coredrive.Drive, error)
}

// Service is the vfs-level drive service. It adds permission
// checks and OpenFGA grant on Create to the core drive.Client.
type Service struct {
	Drive Client
	Perm  permission.Checker
}

// Config groups Service dependencies.
type Config struct {
	Drive Client
	Perm  permission.Checker
}

// NewService wires a Service.
func NewService(cfg Config) *Service {
	return &Service{Drive: cfg.Drive, Perm: cfg.Perm}
}

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

// Get returns a drive by private ID. Requires view permission.
func (s *Service) Get(ctx context.Context, actorID, id string) (*coredrive.Drive, error) {
	if err := checkAccess(ctx, s.Perm, actorID, permission.PermissionView, id); err != nil {
		return nil, err
	}
	return s.Drive.GetByID(ctx, id)
}

// GetByPublicID returns a drive by public ID. No permission check
// (the public ID is the share token).
func (s *Service) GetByPublicID(ctx context.Context, pubID string) (*coredrive.Drive, error) {
	return s.Drive.GetByPublicID(ctx, pubID)
}

// GetStorage returns the storage config for a drive. Requires view.
func (s *Service) GetStorage(ctx context.Context, actorID, driveID string) (*coredrive.Storage, error) {
	if err := checkAccess(ctx, s.Perm, actorID, permission.PermissionView, driveID); err != nil {
		return nil, err
	}
	return s.Drive.GetStorage(ctx, driveID)
}

// Update updates drive fields. Requires edit.
func (s *Service) Update(ctx context.Context, actorID, id string, name, description *string) (*coredrive.Drive, error) {
	if err := checkAccess(ctx, s.Perm, actorID, permission.PermissionEdit, id); err != nil {
		return nil, err
	}
	return s.Drive.Update(ctx, id, name, description)
}

// Delete soft-deletes a drive. Requires delete.
func (s *Service) Delete(ctx context.Context, actorID, id string) error {
	if err := checkAccess(ctx, s.Perm, actorID, permission.PermissionDelete, id); err != nil {
		return err
	}
	return s.Drive.Delete(ctx, id)
}

// Restore reactivates a soft-deleted drive. Requires manage.
func (s *Service) Restore(ctx context.Context, actorID, id string) (*coredrive.Drive, error) {
	if err := checkAccess(ctx, s.Perm, actorID, permission.PermissionManage, id); err != nil {
		return nil, err
	}
	return s.Drive.Restore(ctx, id)
}

// ListByOwner returns all active drives owned by ownerID. The
// caller (typically the handler) is responsible for ensuring
// the requesting user is ownerID; a user can only list their
// own drives. The service does not re-check identity because it
// has no session-layer concept of "the requesting user" — the
// handler is the security boundary. Misuse (passing another
// user's ID) leaks that user's drive listing, so callers must
// enforce.
//
// The method intentionally does not take a separate
// requestingUserID parameter: passing both ownerID and a
// requestingUserID and checking they match would be defensive
// against the footgun, but the check would be unenforceable
// without session context. Document the contract instead.
func (s *Service) ListByOwner(ctx context.Context, ownerID string) ([]*coredrive.Drive, error) {
	return s.Drive.ListByOwner(ctx, ownerID)
}

// ListDeleted returns soft-deleted drives. Admin-only: the caller
// must pass isAdmin=true. The handler is the source of truth for
// "is this user an admin" (via the auth layer); the service
// re-checks here so a non-admin caller cannot bypass the rule
// even if the handler forgets the check.
func (s *Service) ListDeleted(ctx context.Context, isAdmin bool) ([]*coredrive.Drive, error) {
	if !isAdmin {
		return nil, ErrPermission
	}
	return s.Drive.ListDeleted(ctx, time.Now(), 1000)
}

// checkAccess centralizes the vfs-style permission check so the
// Service methods stay focused on the drive operation. A nil Perm
// (development) skips the check.
func checkAccess(ctx context.Context, perm permission.Checker, userID string, p permission.Permission, driveID string) error {
	if perm == nil {
		return nil
	}
	allowed, err := perm.Check(ctx, userID, p, permission.ObjectTypeDrive, driveID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrPermission
	}
	return nil
}

// ErrPermission is returned when a permission check fails.
var ErrPermission = errPermission{}

type errPermission struct{}

func (errPermission) Error() string { return "drive: permission denied" }
