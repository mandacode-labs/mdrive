package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	api "github.com/mandacode-labs/mdrive/pkg/api"
)

// FS is the consumer-declared VFS interface.
type FS interface {
	Mkdir(ctx context.Context, userID, driveID, path string) (*node.Node, error)
	Touch(ctx context.Context, userID, driveID, path string) (*node.Node, error)
	Rm(ctx context.Context, userID, driveID string, paths []string, recursive bool) error
	Mv(ctx context.Context, userID, srcDriveID string, srcPaths []string, dstDriveID, dstPath string) error
	Ls(ctx context.Context, userID, driveID, path string) (node.DirContent, error)
	Cat(ctx context.Context, userID, driveID, path string) ([]byte, error)
	Write(ctx context.Context, userID, driveID, path, content string) error
	WriteLarge(ctx context.Context, userID, driveID, path string, obj node.ObjectContent, size int64) error
	Stat(ctx context.Context, userID, driveID, path string) (*node.Node, error)
	Symlink(ctx context.Context, userID, driveID, target, linkPath string) (*node.Node, error)
	CreateDrive(ctx context.Context, actorID string, name, description string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error)
	GetDrive(ctx context.Context, id string) (*drive.Drive, error)
	GetDriveStorage(ctx context.Context, driveID string) (*drive.Storage, error)
	UpdateDrive(ctx context.Context, id string, name, description *string) (*drive.Drive, error)
	DeleteDrive(ctx context.Context, id string) error
	ListUserDrives(ctx context.Context, actorID string) ([]*drive.Drive, error)
}

// Handler implements the ogen Handler interface.
type Handler struct {
	vfs     FS
	getUser func(context.Context) (string, bool)
}

func New(fs FS, getUser func(context.Context) (string, bool)) *Handler {
	return &Handler{vfs: fs, getUser: getUser}
}

func (h *Handler) userID(ctx context.Context) string {
	id, _ := h.getUser(ctx)
	return id
}

// --- Drive ---

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

// --- FS ---

func (h *Handler) Mkdir(ctx context.Context, req api.OptMkdirReq, params api.MkdirParams) error {
	r := req.Value
	_, err := h.vfs.Mkdir(ctx, h.userID(ctx), params.DriveID, r.Path)
	return err
}

func (h *Handler) Touch(ctx context.Context, req api.OptTouchReq, params api.TouchParams) error {
	r := req.Value
	_, err := h.vfs.Touch(ctx, h.userID(ctx), params.DriveID, r.Path)
	return err
}

func (h *Handler) Rm(ctx context.Context, req api.OptRmReq, params api.RmParams) error {
	r := req.Value
	rec := false
	if r.Recursive.Set {
		rec = bool(r.Recursive.Value)
	}
	return h.vfs.Rm(ctx, h.userID(ctx), params.DriveID, r.Paths, rec)
}

func (h *Handler) Mv(ctx context.Context, req api.OptMvReq, params api.MvParams) error {
	r := req.Value
	return h.vfs.Mv(ctx, h.userID(ctx), params.DriveID, r.Sources, params.DriveID, r.Destination)
}

func (h *Handler) Ls(ctx context.Context, params api.LsParams) (*api.DirContent, error) {
	path := params.Path
	if path == "" {
		path = "/"
	}
	dc, err := h.vfs.Ls(ctx, h.userID(ctx), params.DriveID, path)
	if err != nil {
		return nil, err
	}
	entries := make([]api.DirEntry, len(dc.Entries))
	for i, e := range dc.Entries {
		entries[i] = api.DirEntry{
			InodeID: apistr(e.InodeID.String()),
			Name:    apistr(e.Name),
			Type:    apistr(e.Type.String()),
		}
	}
	return &api.DirContent{Entries: entries}, nil
}

func (h *Handler) Cat(ctx context.Context, params api.CatParams) (api.CatOK, error) {
	data, err := h.vfs.Cat(ctx, h.userID(ctx), params.DriveID, params.Path)
	if err != nil {
		return api.CatOK{}, err
	}
	return api.CatOK{Data: bytes.NewReader(data)}, nil
}

func (h *Handler) Write(ctx context.Context, req api.OptWriteReq, params api.WriteParams) error {
	r := req.Value
	return h.vfs.Write(ctx, h.userID(ctx), params.DriveID, r.Path, r.Content)
}

func (h *Handler) WriteLarge(ctx context.Context, req api.OptWriteLargeReq, params api.WriteLargeParams) error {
	r := req.Value
	ct := ""
	cs := ""
	if r.Object.ContentType.Set {
		ct = r.Object.ContentType.Value
	}
	if r.Object.Checksum.Set {
		cs = r.Object.Checksum.Value
	}
	obj := node.ObjectContent{
		Bucket:   r.Object.Bucket,
		Key:      r.Object.Key,
		Mime:     ct,
		Checksum: cs,
	}
	return h.vfs.WriteLarge(ctx, h.userID(ctx), params.DriveID, r.Path, obj, r.Size)
}

func (h *Handler) Symlink(ctx context.Context, req api.OptSymlinkReq, params api.SymlinkParams) error {
	r := req.Value
	_, err := h.vfs.Symlink(ctx, h.userID(ctx), params.DriveID, r.Target, r.LinkPath)
	return err
}

func (h *Handler) Stat(ctx context.Context, params api.StatParams) (*api.StatOK, error) {
	n, err := h.vfs.Stat(ctx, h.userID(ctx), params.DriveID, params.Path)
	if err != nil {
		return nil, err
	}
	return &api.StatOK{
		Type:     apistr(n.Type().String()),
		Size:     api.OptInt64{Value: n.Size(), Set: true},
		Atime:    api.OptDateTime{Value: n.ATime(), Set: true},
		Mtime:    api.OptDateTime{Value: n.MTime(), Set: true},
		Ctime:    api.OptDateTime{Value: n.CTime(), Set: true},
		Flags:    apistr(n.Flags().String()),
		Revision: apistr(n.Revision().String()),
	}, nil
}

// --- Helpers ---

func driveToAPI(d *drive.Drive) *api.Drive {
	if d == nil {
		return nil
	}
	rid := d.RootNodeID()
	var rids string
	if rid != nil {
		rids = rid.String()
	}
	return &api.Drive{
		ID:          apistr(d.ID()),
		PublicID:    apistr(d.PublicID()),
		Name:        apistr(d.Name()),
		Description: apistrPtr(d.Description()),
		OwnerID:     apistr(d.OwnerID()),
		RootNodeID:  apistr(rids),
		CreatedAt:   api.OptDateTime{Value: d.CreatedAt(), Set: true},
		UpdatedAt:   api.OptDateTime{Value: d.UpdatedAt(), Set: true},
	}
}

func apistr(s string) api.OptString {
	return api.OptString{Value: s, Set: true}
}

func apistrPtr(s *string) api.OptString {
	if s == nil {
		return api.OptString{}
	}
	return api.OptString{Value: *s, Set: true}
}

func optBool(b bool) api.OptBool {
	return api.OptBool{Value: b, Set: true}
}

// Compile-time check.
var _ api.Handler = (*Handler)(nil)
var _ io.Reader = (*bytes.Reader)(nil)
var _ = fmt.Sprintf
