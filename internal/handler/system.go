package handler

import (
	"context"

	api "github.com/mandacode-labs/mdrive/pkg/api"

	coreuser "github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/errors"
	"github.com/mandacode-labs/mdrive/internal/service/sysinit"
	"github.com/mandacode-labs/mdrive/internal/system"
	"github.com/mandacode-labs/mdrive/internal/utils"
)

// CreateSystem implements POST /systems.
func (h *Handler) CreateSystem(ctx context.Context, req *api.CreateSystemRequest) (api.CreateSystemRes, error) {
	userID, ok := utils.GetUserID(ctx)
	if !ok {
		return nil, h.domainError(errors.Unauthorized("user not authenticated"))
	}

	var description *string
	if req.Description.Set {
		description = &req.Description.Value
	}

	result, err := h.initSvc.InitSystem(ctx, &sysinit.InitSystemCommand{
		Name:        req.Name,
		Description: description,
		RootUserID:  userID,
	})
	if err != nil {
		return nil, h.domainError(err)
	}

	return &api.SystemResponse{
		System: *h.toSystem(result.System),
	}, nil
}

// ListSystems implements GET /systems.
func (h *Handler) ListSystems(ctx context.Context) (api.ListSystemsRes, error) {
	userID, ok := utils.GetUserID(ctx)
	if !ok {
		return nil, h.domainError(errors.Unauthorized("user not authenticated"))
	}

	// Find all system memberships for this user
	memberships, err := h.sysUserSvc.Find(ctx, coreuser.ByUserID(userID))
	if err != nil {
		return nil, h.domainError(err)
	}

	// Collect system IDs
	systemIDs := make([]string, len(memberships))
	for i, m := range memberships {
		systemIDs[i] = m.SystemID()
	}

	// Load each system
	resp := &api.SystemListResponse{
		Systems: make([]api.System, 0, len(systemIDs)),
	}
	for _, sysID := range systemIDs {
		sys, err := h.systemSvc.GetByID(ctx, sysID)
		if err != nil {
			continue // Skip systems that may have been deleted
		}
		resp.Systems = append(resp.Systems, *h.toSystem(sys))
	}

	return resp, nil
}

// GetSystem implements GET /systems/{systemId}.
func (h *Handler) GetSystem(ctx context.Context, params api.GetSystemParams) (api.GetSystemRes, error) {
	sys, err := h.systemSvc.GetByID(ctx, params.SystemId)
	if err != nil {
		return nil, h.domainError(err)
	}

	return &api.SystemResponse{
		System: *h.toSystem(sys),
	}, nil
}

func (h *Handler) DeleteSystem(ctx context.Context, params api.DeleteSystemParams) (api.DeleteSystemRes, error) {
	if err := h.checkSystemAccess(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	if err := h.systemSvc.Delete(ctx, params.SystemId); err != nil {
		return nil, h.domainError(err)
	}

	return &api.DeleteSystemNoContent{}, nil
}

func (h *Handler) toSystem(sys *system.System) *api.System {
	resp := &api.System{
		ID:        sys.ID(),
		Name:      sys.Name(),
		Status:    api.SystemStatus(sys.Status()),
		CreatedAt: toOptTimestamp(sys.CreatedAt()),
		UpdatedAt: toOptTimestamp(sys.UpdatedAt()),
	}
	if desc := sys.Description(); desc != nil {
		resp.Description.Set = true
		resp.Description.Value = *desc
	}
	return resp
}
