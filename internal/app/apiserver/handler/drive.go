package handler

import (
	"context"
	"time"

	"github.com/mandacode-labs/mdrive/internal/app/apputils"
	"github.com/mandacode-labs/mdrive/internal/auth"
	coredrive "github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

func driveToAPI(d *coredrive.Drive) *api.Drive {
	if d == nil {
		return nil
	}
	rid := d.RootNodeID()
	var rids string
	if rid != nil {
		rids = rid.String()
	}
	deletedAt := api.OptDateTime{}
	if d.DeletedAt() != nil {
		deletedAt = api.OptDateTime{Value: *d.DeletedAt(), Set: true}
	}
	return &api.Drive{
		ID:          apputils.OptString(d.ID()),
		PublicID:    apputils.OptString(d.PublicID()),
		Name:        apputils.OptString(d.Name()),
		Description: apputils.OptStringPtr(d.Description()),
		OwnerID:     apputils.OptString(d.OwnerID()),
		RootNodeID:  apputils.OptString(rids),
		DeletedAt:   deletedAt,
		CreatedAt:   api.OptDateTime{Value: d.CreatedAt(), Set: true},
		UpdatedAt:   api.OptDateTime{Value: d.UpdatedAt(), Set: true},
	}
}

// --- Drive handlers ---

func (h *Handler) CreateDrive(ctx context.Context, req api.OptDriveCreate) (api.CreateDriveRes, error) {
	r := req.Value
	desc := ""
	if r.Description.Set {
		desc = r.Description.Value
	}
	cfg := h.defaultStorage
	if r.Storage.Set {
		override := r.Storage.Value
		cfg = storageConfigFromAPI(override)
	}
	d, _, err := h.drive.Create(ctx, h.userID(ctx), r.Name, desc, cfg)
	if err != nil {
		return nil, err
	}
	return driveToAPI(d), nil
}

// storageConfigFromAPI converts an api.StorageConfig into a coredrive.StorageConfig.
// Nil/empty optional fields are propagated as such; empty strings fall back to
// the platform default for bucket/region to preserve prior behavior.
func storageConfigFromAPI(s api.StorageConfig) coredrive.StorageConfig {
	bucket := s.Bucket
	region := s.Region
	endpoint := s.Endpoint.Value
	accessKey := s.AccessKey.Value
	secretKey := s.SecretKey.Value
	usePathStyle := false
	if s.UsePathStyle.Set {
		usePathStyle = s.UsePathStyle.Value
	}
	var endpointPtr *string
	if endpoint != "" {
		endpointPtr = &endpoint
	}
	return coredrive.StorageConfig{
		Bucket:       bucket,
		Endpoint:     endpointPtr,
		Region:       region,
		AccessKey:    accessKey,
		SecretKey:    secretKey,
		UsePathStyle: usePathStyle,
	}
}

func (h *Handler) GetDrive(ctx context.Context, params api.GetDriveParams) (api.GetDriveRes, error) {
	if err := h.requirePerm(ctx, permission.ActionView, params.DriveID); err != nil {
		return nil, err
	}
	d, err := h.drive.GetByID(ctx, params.DriveID)
	if err != nil {
		return nil, err
	}
	return driveToAPI(d), nil
}

func (h *Handler) UpdateDrive(ctx context.Context, req api.OptDriveUpdate, params api.UpdateDriveParams) (api.UpdateDriveRes, error) {
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	r := req.Value
	var name, desc string
	if r.Name.Set {
		name = r.Name.Value
	}
	if r.Description.Set {
		desc = r.Description.Value
	}
	drv, err := h.drive.Update(ctx, h.userID(ctx), params.DriveID, name, desc)
	if err != nil {
		return nil, err
	}
	return driveToAPI(drv), nil
}

func (h *Handler) DeleteDrive(ctx context.Context, params api.DeleteDriveParams) (api.DeleteDriveRes, error) {
	if err := h.requirePerm(ctx, permission.ActionEdit, params.DriveID); err != nil {
		return nil, err
	}
	if err := h.drive.Delete(ctx, h.userID(ctx), params.DriveID); err != nil {
		return nil, err
	}
	return &api.DeleteDriveNoContent{}, nil
}

func (h *Handler) RestoreDrive(ctx context.Context, params api.RestoreDriveParams) (api.RestoreDriveRes, error) {
	if !auth.IsAdmin(ctx) {
		return nil, permission.ErrPermission
	}
	d, err := h.drive.Restore(ctx, h.userID(ctx), params.DriveID)
	if err != nil {
		return nil, err
	}
	return driveToAPI(d), nil
}

func (h *Handler) ListDeletedDrives(ctx context.Context) (api.ListDeletedDrivesRes, error) {
	if !auth.IsAdmin(ctx) {
		return nil, permission.ErrPermission
	}
	drives, err := h.drive.ListDeletedForAdmin(ctx, true, time.Now(), 1000)
	if err != nil {
		return nil, err
	}
	result := make([]api.Drive, len(drives))
	for i, d := range drives {
		result[i] = *driveToAPI(d)
	}
	r := api.ListDeletedDrivesOKApplicationJSON(result)
	return &r, nil
}

func (h *Handler) GetDriveStorage(ctx context.Context, params api.GetDriveStorageParams) (api.GetDriveStorageRes, error) {
	if err := h.requirePerm(ctx, permission.ActionView, params.DriveID); err != nil {
		return nil, err
	}
	s, err := h.drive.GetStorage(ctx, params.DriveID)
	if err != nil {
		return nil, err
	}
	// Return only non-credential fields. AccessKey/SecretKey are
	// write-only and must not leak through a read endpoint.
	return &api.StorageConfig{
		Bucket:       s.Bucket(),
		Region:       s.Region(),
		Endpoint:     apputils.OptStringPtr(s.Endpoint()),
		UsePathStyle: apputils.OptBool(s.UsePathStyle()),
	}, nil
}

func (h *Handler) ListDrives(ctx context.Context) (api.ListDrivesRes, error) {
	drives, err := h.drive.ListByOwner(ctx, h.userID(ctx))
	if err != nil {
		return nil, err
	}
	result := make([]api.Drive, len(drives))
	for i, d := range drives {
		result[i] = *driveToAPI(d)
	}
	r := api.ListDrivesOKApplicationJSON(result)
	return &r, nil
}
