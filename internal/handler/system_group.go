package handler

import (
	"context"

	api "github.com/mandacode-labs/retrowin-go/pkg/api"

	coreuser "github.com/mandacode-labs/retrowin-go/internal/core/user"
)

// CreateSystemGroup implements POST /systems/{systemId}/groups.
func (h *Handler) CreateSystemGroup(ctx context.Context, req *api.CreateSystemGroupRequest, params api.CreateSystemGroupParams) (api.CreateSystemGroupRes, error) {
	cmd := &coreuser.GroupCreateCommand{
		SystemID: params.SystemId,
		Name:     req.Name,
		GID:      -1, // Default to -1 for auto-assignment
	}
	if req.Gid.Set {
		cmd.GID = int(req.Gid.Value)
	}

	group, err := h.sysGroupSvc.Create(ctx, cmd)
	if err != nil {
		return nil, h.domainError(err)
	}

	return &api.SystemGroupResponse{
		Group: *h.toSystemGroup(group),
	}, nil
}

// ListSystemGroups implements GET /systems/{systemId}/groups.
func (h *Handler) ListSystemGroups(ctx context.Context, params api.ListSystemGroupsParams) (api.ListSystemGroupsRes, error) {
	groups, err := h.sysGroupSvc.Find(ctx, coreuser.GroupBySystemID(params.SystemId))
	if err != nil {
		return nil, h.domainError(err)
	}

	resp := &api.SystemGroupListResponse{
		Groups: make([]api.SystemGroup, len(groups)),
	}
	for i, g := range groups {
		resp.Groups[i] = *h.toSystemGroup(g)
	}

	return resp, nil
}

// GetSystemGroup implements GET /systems/{systemId}/groups/{gid}.
func (h *Handler) GetSystemGroup(ctx context.Context, params api.GetSystemGroupParams) (api.GetSystemGroupRes, error) {
	group, err := h.sysGroupSvc.FindOne(ctx, coreuser.GroupBySystemAndGID(params.SystemId, int(params.Gid)))
	if err != nil {
		return nil, h.domainError(err)
	}

	return &api.SystemGroupResponse{
		Group: *h.toSystemGroup(group),
	}, nil
}

// DeleteSystemGroup implements DELETE /systems/{systemId}/groups/{gid}.
func (h *Handler) DeleteSystemGroup(ctx context.Context, params api.DeleteSystemGroupParams) (api.DeleteSystemGroupRes, error) {
	// Find group first
	group, err := h.sysGroupSvc.FindOne(ctx, coreuser.GroupBySystemAndGID(params.SystemId, int(params.Gid)))
	if err != nil {
		return nil, h.domainError(err)
	}

	if err := h.sysGroupSvc.Delete(ctx, group.ID()); err != nil {
		return nil, h.domainError(err)
	}

	return &api.DeleteSystemGroupNoContent{}, nil
}

// AddGroupMember implements POST /systems/{systemId}/groups/{gid}/members/{uid}.
func (h *Handler) AddGroupMember(ctx context.Context, params api.AddGroupMemberParams) (api.AddGroupMemberRes, error) {
	// Find group first
	group, err := h.sysGroupSvc.FindOne(ctx, coreuser.GroupBySystemAndGID(params.SystemId, int(params.Gid)))
	if err != nil {
		return nil, h.domainError(err)
	}

	// Find user by UID within system
	targetUser, err := h.sysUserSvc.FindOne(ctx, coreuser.BySystemIDAndUID(params.SystemId, int(params.UID)))
	if err != nil {
		return nil, h.domainError(err)
	}

	if err := h.sysGroupSvc.AddUserToGroup(ctx, targetUser.ID(), group.ID()); err != nil {
		return nil, h.domainError(err)
	}

	return &api.AddGroupMemberNoContent{}, nil
}

// RemoveGroupMember implements DELETE /systems/{systemId}/groups/{gid}/members/{uid}.
func (h *Handler) RemoveGroupMember(ctx context.Context, params api.RemoveGroupMemberParams) (api.RemoveGroupMemberRes, error) {
	// Find group first
	group, err := h.sysGroupSvc.FindOne(ctx, coreuser.GroupBySystemAndGID(params.SystemId, int(params.Gid)))
	if err != nil {
		return nil, h.domainError(err)
	}

	// Find user by UID within system
	targetUser, err := h.sysUserSvc.FindOne(ctx, coreuser.BySystemIDAndUID(params.SystemId, int(params.UID)))
	if err != nil {
		return nil, h.domainError(err)
	}

	if err := h.sysGroupSvc.RemoveUserFromGroup(ctx, targetUser.ID(), group.ID()); err != nil {
		return nil, h.domainError(err)
	}

	return &api.RemoveGroupMemberNoContent{}, nil
}

func (h *Handler) toSystemGroup(g *coreuser.SystemGroup) *api.SystemGroup {
	return &api.SystemGroup{
		ID:       int64(g.ID()),
		SystemId: g.SystemID(),
		Name:     g.Name(),
		Gid:      int32(g.GID()),
	}
}
