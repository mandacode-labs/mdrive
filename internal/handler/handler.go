package handler

import (
	"context"
	"time"

	api "github.com/mandacode-labs/retrowin-go/pkg/api"

	"github.com/mandacode-labs/retrowin-go/internal/application/storage"
	corevfs "github.com/mandacode-labs/retrowin-go/internal/application/vfs"
	"github.com/mandacode-labs/retrowin-go/internal/auth"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
	"github.com/mandacode-labs/retrowin-go/internal/service/sysinit"
	"github.com/mandacode-labs/retrowin-go/internal/system"
	extuser "github.com/mandacode-labs/retrowin-go/internal/user"
	"github.com/mandacode-labs/retrowin-go/internal/utils"
)

// Handler implements the ogen API handler interface.
type Handler struct {
	// Auth service
	authSvc auth.AuthService

	// External user service (for /user endpoints)
	extUserSvc extuser.UserService

	// System user/group services (for /systems/{id}/users and /systems/{id}/groups endpoints)
	sysUserSvc  UserService
	sysGroupSvc GroupService

	// System service
	systemSvc system.SystemService

	// Filesystem and storage services
	fsSvc      corevfs.VFS
	inodeOps   InodeService
	storageSvc storage.StorageService

	// Init service
	initSvc sysinit.InitService
}

// NewHandler creates a new Handler.
func NewHandler(
	authSvc auth.AuthService,
	extUserSvc extuser.UserService,
	sysUserSvc UserService,
	sysGroupSvc GroupService,
	systemSvc system.SystemService,
	fsSvc corevfs.VFS,
	inodeOps InodeService,
	storageSvc storage.StorageService,
	initSvc sysinit.InitService,
) *Handler {
	return &Handler{
		authSvc:     authSvc,
		extUserSvc:  extUserSvc,
		sysUserSvc:  sysUserSvc,
		sysGroupSvc: sysGroupSvc,
		systemSvc:   systemSvc,
		fsSvc:       fsSvc,
		inodeOps:    inodeOps,
		storageSvc:  storageSvc,
		initSvc:     initSvc,
	}
}

// Ensure Handler implements the ogen Handler interface.
var _ api.Handler = (*Handler)(nil)

// GetUser implements GET /user.
func (h *Handler) GetUser(ctx context.Context) (api.GetUserRes, error) {
	userID, ok := utils.GetUserID(ctx)
	if !ok {
		return &api.GetUserUnauthorized{}, nil
	}

	u, err := h.extUserSvc.GetByID(ctx, userID)
	if err != nil {
		return nil, h.domainError(err)
	}

	return &api.UserResponse{
		User: api.User{
			ID:         u.ID(),
			Provider:   api.Provider(u.Provider()),
			ProviderId: u.ProviderID(),
		},
	}, nil
}

// DeleteUser implements DELETE /user.
func (h *Handler) DeleteUser(ctx context.Context) (api.DeleteUserRes, error) {
	return &api.DeleteUserNoContent{}, nil
}

// Helper functions

func toTimestamp(t time.Time) api.Timestamp {
	return api.Timestamp(t)
}

func toOptTimestamp(t time.Time) api.OptTimestamp {
	return api.NewOptTimestamp(toTimestamp(t))
}

// domainError converts domain errors to HTTP errors.
func (h *Handler) domainError(err error) error {
	return err // ErrorHandler/NewError in errors.go handles status code mapping
}

// checkSystemAccess verifies that the authenticated user has access to the given system.
func (h *Handler) checkSystemAccess(ctx context.Context, systemID string) error {
	userID, ok := utils.GetUserID(ctx)
	if !ok {
		return errors.Unauthorized("user not authenticated")
	}

	_, err := h.sysUserSvc.GetByUserAndSystem(ctx, userID, systemID)
	if err != nil {
		return errors.Forbidden("access denied to system")
	}
	return nil
}
