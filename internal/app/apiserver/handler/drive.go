package handler

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// --- Drive handlers ---

func (h *Handler) CreateDrive(ctx context.Context, req api.OptDriveCreate) (api.CreateDriveRes, error) {
	r := req.Value
	desc := ""
	if r.Description.Set {
		desc = r.Description.Value
	}
	ep := (*string)(nil)
	if r.Storage.Endpoint.Set {
		ep = &r.Storage.Endpoint.Value
	}
	cfg := drive.StorageConfig{
		Bucket:       r.Storage.Bucket,
		Endpoint:     ep,
		Region:       r.Storage.Region,
		AccessKey:    r.Storage.AccessKey,
		SecretKey:    r.Storage.SecretKey,
		UsePathStyle: r.Storage.UsePathStyle.Value && r.Storage.UsePathStyle.Set,
	}
	d, _, err := h.vfs.CreateDrive(ctx, h.userID(ctx), r.Name, desc, cfg)
	if err != nil {
		return nil, err
	}
	return driveToAPI(d), nil
}

func (h *Handler) GetDrive(ctx context.Context, params api.GetDriveParams) (*api.Drive, error) {
	d, err := h.vfs.GetDrive(ctx, params.DriveID)
	if err != nil {
		return nil, err
	}
	return driveToAPI(d), nil
}

func (h *Handler) UpdateDrive(ctx context.Context, req api.OptDriveUpdate, params api.UpdateDriveParams) (*api.Drive, error) {
	r := req.Value
	n := (*string)(nil)
	d := (*string)(nil)
	if r.Name.Set {
		n = &r.Name.Value
	}
	if r.Description.Set {
		d = &r.Description.Value
	}
	drv, err := h.vfs.UpdateDrive(ctx, params.DriveID, n, d)
	if err != nil {
		return nil, err
	}
	return driveToAPI(drv), nil
}

func (h *Handler) DeleteDrive(ctx context.Context, params api.DeleteDriveParams) error {
	return h.vfs.DeleteDrive(ctx, params.DriveID)
}

func (h *Handler) GetDriveStorage(ctx context.Context, params api.GetDriveStorageParams) (*api.StorageConfig, error) {
	s, err := h.vfs.GetDriveStorage(ctx, params.DriveID)
	if err != nil {
		return nil, err
	}
	return &api.StorageConfig{
		Bucket:       s.Bucket(),
		Region:       s.Region(),
		AccessKey:    s.AccessKey(),
		SecretKey:    s.SecretKey(),
		Endpoint:     apistrPtr(s.Endpoint()),
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
