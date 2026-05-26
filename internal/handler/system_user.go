package handler

import (
	"context"

	api "github.com/mandacode-labs/retrowin-go/pkg/api"

	coreuser "github.com/mandacode-labs/retrowin-go/internal/core/user"
)

// CreateSystemUser implements POST /systems/{systemId}/users.
func (h *Handler) CreateSystemUser(ctx context.Context, req *api.CreateSystemUserRequest, params api.CreateSystemUserParams) (api.CreateSystemUserRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	cmd := &coreuser.CreateCommand{
		UserID:   req.UserId,
		SystemID: params.SystemId,
		Username: req.Username,
		UID:      -1, // Default to auto-assign
	}
	if req.UID.Set {
		cmd.UID = int(req.UID.Value)
	}

	sysUser, err := h.sysUserSvc.Create(ctx, cmd)
	if err != nil {
		return nil, h.domainError(err)
	}

	return &api.SystemUserResponse{
		User: *h.toSystemUser(sysUser),
	}, nil
}

// ListSystemUsers implements GET /systems/{systemId}/users.
func (h *Handler) ListSystemUsers(ctx context.Context, params api.ListSystemUsersParams) (api.ListSystemUsersRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	users, err := h.sysUserSvc.Find(ctx, coreuser.BySystemID(params.SystemId))
	if err != nil {
		return nil, h.domainError(err)
	}

	resp := &api.SystemUserListResponse{
		Users: make([]api.SystemUser, len(users)),
	}
	for i, u := range users {
		resp.Users[i] = *h.toSystemUser(u)
	}

	return resp, nil
}

// GetSystemUser implements GET /systems/{systemId}/users/{uid}.
func (h *Handler) GetSystemUser(ctx context.Context, params api.GetSystemUserParams) (api.GetSystemUserRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	uid := int(params.UID)
	user, err := h.sysUserSvc.FindOne(ctx, coreuser.BySystemIDAndUID(params.SystemId, uid))
	if err != nil {
		return nil, h.domainError(err)
	}
	if user == nil {
		return &api.GetSystemUserNotFound{
			Error: api.ErrorError{
				Type:    "not_found",
				Message: "system user not found",
			},
		}, nil
	}

	return &api.SystemUserResponse{
		User: *h.toSystemUser(user),
	}, nil
}

// DeleteSystemUser implements DELETE /systems/{systemId}/users/{uid}.
func (h *Handler) DeleteSystemUser(ctx context.Context, params api.DeleteSystemUserParams) (api.DeleteSystemUserRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	uid := int(params.UID)
	targetUser, err := h.sysUserSvc.FindOne(ctx, coreuser.BySystemIDAndUID(params.SystemId, uid))
	if err != nil {
		return nil, h.domainError(err)
	}

	if targetUser == nil {
		return &api.DeleteSystemUserNotFound{
			Error: api.ErrorError{
				Type:    "not_found",
				Message: "system user not found",
			},
		}, nil
	}

	if err := h.sysUserSvc.Delete(ctx, targetUser.ID()); err != nil {
		return nil, h.domainError(err)
	}

	return &api.DeleteSystemUserNoContent{}, nil
}

func (h *Handler) toSystemUser(u *coreuser.SystemUser) *api.SystemUser {
	return &api.SystemUser{
		ID:       int64(u.ID()),
		UserId:   u.UserID(),
		SystemId: u.SystemID(),
		Username: u.Username(),
		UID:      int32(u.UID()),
		Gid:      int32(u.GID()),
	}
}
