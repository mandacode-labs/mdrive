package handler

import (
	"context"

	"github.com/mandacode-labs/mdrive/pkg/api"
)

// --- Drive handlers ---

func (h *Handler) CreateDrive(ctx context.Context, req api.OptDriveCreate) (api.CreateDriveRes, error) {
	r := req.Value
	desc := ""
	if r.Description.Set {
		desc = r.Description.Value
	}
	// Custom drive storage is disabled; always use the platform default storage.
	d, _, err := h.vfs.CreateDrive(ctx, h.userID(ctx), r.Name, desc, h.defaultStorage)
	if err != nil {
		return nil, err
	}
	return driveToAPI(d), nil
}

func (h *Handler) GetDrive(ctx context.Context, params api.GetDriveParams) (*api.Drive, error) {
	d, err := h.vfs.GetDrive(ctx, h.userID(ctx), params.DriveID)
	if err != nil {
		return nil, err
	}
	return driveToAPI(d), nil
}

func (h *Handler) UpdateDrive(ctx context.Context, req api.OptDriveUpdate, params api.UpdateDriveParams) (*api.Drive, error) {
	r := req.Value
	namePtr := (*string)(nil)
	descPtr := (*string)(nil)
	if r.Name.Set {
		namePtr = &r.Name.Value
	}
	if r.Description.Set {
		descPtr = &r.Description.Value
	}
	drv, err := h.vfs.UpdateDrive(ctx, h.userID(ctx), params.DriveID, namePtr, descPtr)
	if err != nil {
		return nil, err
	}
	return driveToAPI(drv), nil
}

func (h *Handler) DeleteDrive(ctx context.Context, params api.DeleteDriveParams) error {
	return h.vfs.DeleteDrive(ctx, h.userID(ctx), params.DriveID)
}

func (h *Handler) RestoreDrive(ctx context.Context, params api.RestoreDriveParams) (*api.Drive, error) {
	d, err := h.vfs.RestoreDrive(ctx, h.userID(ctx), params.DriveID)
	if err != nil {
		return nil, err
	}
	return driveToAPI(d), nil
}

func (h *Handler) ListDeletedDrives(ctx context.Context) ([]api.Drive, error) {
	drives, err := h.vfs.ListDeletedDrives(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]api.Drive, len(drives))
	for i, d := range drives {
		result[i] = *driveToAPI(d)
	}
	return result, nil
}

func (h *Handler) GetDriveStorage(ctx context.Context, params api.GetDriveStorageParams) (*api.StorageConfig, error) {
	s, err := h.vfs.GetDriveStorage(ctx, h.userID(ctx), params.DriveID)
	if err != nil {
		return nil, err
	}
	return &api.StorageConfig{
		Bucket:       s.Bucket(),
		Region:       s.Region(),
		Endpoint:     toOptStringPtr(s.Endpoint()),
		UsePathStyle: optBool(s.UsePathStyle()),
	}, nil
}

func (h *Handler) ListDrives(ctx context.Context) ([]api.Drive, error) {
	drives, err := h.vfs.ListUserDrives(ctx, h.userID(ctx))
	if err != nil {
		return nil, err
	}
	result := make([]api.Drive, len(drives))
	for i, d := range drives {
		result[i] = *driveToAPI(d)
	}
	return result, nil
}
