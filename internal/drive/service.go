package drive

import (
	"context"

	coredrive "github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

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

// checkAccess centralizes the vfs-style permission check so the
// Service methods stay focused on the drive operation. A nil Perm
// (development) skips the check.
func (s *Service) checkAccess(ctx context.Context, userID string, p permission.Permission, driveID string) error {
	if s.Perm == nil {
		return nil
	}
	allowed, err := s.Perm.Check(ctx, userID, p, permission.ObjectTypeDrive, driveID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrPermission
	}
	return nil
}

// compile-time interface check
var _ = (*coredrive.Drive)(nil)
