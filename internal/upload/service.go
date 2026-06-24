package upload

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/google/uuid"

	coredrive "github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// PresignInfo is the result of initiating or presigning a download URL.
type PresignInfo struct {
	UploadID  string
	Method    string
	URL       string
	Headers   map[string]string
	Key       string
	ExpiresAt time.Time
}

// DriveLookup is the data-access contract for the storage config
// of a drive. The underlying *drive.Service satisfies it via
// GetStorage; the actorID parameter is unused (the storage
// record is fetched unconditionally) but the signature is kept
// uniform with the rest of the drive service.
type DriveLookup interface {
	GetStorage(ctx context.Context, actorID, driveID string) (*coredrive.Storage, error)
}

// NodeOps is the subset of node.Service the upload flow needs:
// create object node, link into parent, delete on failure, and
// look up an existing node by ID.
type NodeOps interface {
	CreateObject(ctx context.Context, content node.ObjectContent, size int64) (*node.Node, error)
	Link(ctx context.Context, parent *node.Node, name string, child *node.Node) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*node.Node, error)
}

// ObjectStore is the S3 abstraction the upload flow needs: presigned
// URLs, object existence check, and direct delete. The DeleteObject
// method is exposed because gc.UploadExpirer uses it to clean up
// objects the client uploaded but never completed.
type ObjectStore interface {
	GetPresignedUploadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
	GetPresignedDownloadURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
	ObjectExists(ctx context.Context, bucket, key string) (bool, error)
	DeleteObject(ctx context.Context, bucket, key string) error
}

// PathResolver provides drive-root lookup and path resolution.
// vfs.Service satisfies it; tests can pass fakes.
type PathResolver interface {
	GetRootNodeID(ctx context.Context, driveID string) (uuid.UUID, error)
	ResolveParentNodeID(ctx context.Context, driveID, path string) (uuid.UUID, string, error)
	ResolveNodeID(ctx context.Context, driveID, path string) (uuid.UUID, error)
}

// Service is the vfs-level upload orchestrator. It composes the
// upload Registry (token storage) with the vfs primitives (node
// tree, path resolution) and the S3 store.
//
// Permission checks are the caller's responsibility: the handler
// must call permission.Require on the drive before invoking
// any of the three methods below.
type Service struct {
	Reg   Registry
	Drive DriveLookup
	Nodes NodeOps
	Store ObjectStore
	Path  PathResolver
}

// Config groups Service dependencies.
type Config struct {
	Reg   Registry
	Drive DriveLookup
	Nodes NodeOps
	Store ObjectStore
	Path  PathResolver
}

// NewService wires a Service. A nil Reg defaults to a MemoryRegistry
// (in-process, no TTL eviction); production code should pass a
// Valkey-backed registry.
func NewService(cfg Config) *Service {
	if cfg.Reg == nil {
		cfg.Reg = NewMemoryRegistry()
	}
	return &Service{
		Reg:   cfg.Reg,
		Drive: cfg.Drive,
		Nodes: cfg.Nodes,
		Store: cfg.Store,
		Path:  cfg.Path,
	}
}

// InitiateUpload creates a presigned PUT URL for uploading an object directly to S3.
// The returned uploadID must be passed to CompleteUpload after the client PUTs the bytes.
// Permission is the caller's responsibility.
func (s *Service) InitiateUpload(ctx context.Context, userID, driveID, destPath string, contentType *string, contentLength *int64, expiry time.Duration) (PresignInfo, error) {
	_ = userID
	storage, err := s.Drive.GetStorage(ctx, userID, driveID)
	if err != nil {
		return PresignInfo{}, err
	}
	bucket := storage.Bucket()
	if bucket == "" {
		return PresignInfo{}, fmt.Errorf("initiate upload: drive has no bucket configured")
	}
	uploadID := uuid.NewString()
	key := path.Join("drives", driveID, "uploads", uploadID)

	meta := PresignMeta{
		UploadID:    uploadID,
		DriveID:     driveID,
		UserID:      userID,
		Path:        destPath,
		Bucket:      bucket,
		Key:         key,
		ContentType: contentType,
		Size:        contentLength,
		ExpiresAt:   time.Now().Add(expiry),
	}
	// Write to registry FIRST — if this fails, no presigned URL is issued.
	if err := s.Reg.Put(ctx, meta, expiry); err != nil {
		return PresignInfo{}, fmt.Errorf("initiate upload: register: %w", err)
	}

	url, err := s.Store.GetPresignedUploadURL(ctx, bucket, key, expiry)
	if err != nil {
		_ = s.Reg.Delete(ctx, uploadID)
		return PresignInfo{}, fmt.Errorf("initiate upload: presign: %w", err)
	}

	return PresignInfo{
		UploadID:  uploadID,
		Method:    "PUT",
		URL:       url,
		Headers:   map[string]string{},
		Key:       key,
		ExpiresAt: meta.ExpiresAt,
	}, nil
}

// CompleteUpload validates the upload token, verifies the S3 object exists,
// and creates the object node at the destination path.
// Permission is the caller's responsibility.
func (s *Service) CompleteUpload(ctx context.Context, userID, driveID, uploadID string, contentLength int64, checksum *string) (*node.Node, error) {
	_ = userID
	meta, err := s.Reg.Get(ctx, uploadID)
	if err != nil {
		return nil, fmt.Errorf("complete upload: get token: %w", err)
	}
	if meta.DriveID != driveID {
		return nil, ErrUploadMismatch
	}
	if meta.Size != nil && *meta.Size != contentLength {
		return nil, fmt.Errorf("complete upload: size mismatch: expected %d, got %d", *meta.Size, contentLength)
	}

	exists, err := s.Store.ObjectExists(ctx, meta.Bucket, meta.Key)
	if err != nil {
		return nil, fmt.Errorf("complete upload: check object: %w", err)
	}
	if !exists {
		return nil, ErrObjectNotUploaded
	}

	parentID, name, err := s.Path.ResolveParentNodeID(ctx, driveID, meta.Path)
	if err != nil {
		return nil, fmt.Errorf("complete upload: %w", err)
	}
	parent, err := s.Nodes.GetByID(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("complete upload: load parent: %w", err)
	}

	var ct string
	if meta.ContentType != nil {
		ct = *meta.ContentType
	}
	var cs string
	if checksum != nil {
		cs = *checksum
	}
	obj := node.ObjectContent{
		Bucket:   meta.Bucket,
		Key:      meta.Key,
		Mime:     ct,
		Checksum: cs,
	}
	n, err := s.Nodes.CreateObject(ctx, obj, contentLength)
	if err != nil {
		return nil, err
	}
	if lerr := s.Nodes.Link(ctx, parent, name, n); lerr != nil {
		if derr := s.Nodes.Delete(ctx, n.ID()); derr != nil {
			return nil, fmt.Errorf("complete upload: link: %w (cleanup: %v)", lerr, derr)
		}
		return nil, fmt.Errorf("complete upload: link: %w", lerr)
	}
	if err := s.Reg.Delete(ctx, uploadID); err != nil {
		return n, fmt.Errorf("complete upload: cleanup token: %w", err)
	}
	return n, nil
}

// PresignDownload returns a presigned GET URL for an existing object node.
// Permission is the caller's responsibility.
func (s *Service) PresignDownload(ctx context.Context, userID, driveID, filePath string, expiry time.Duration) (PresignInfo, error) {
	_ = userID
	nodeID, err := s.Path.ResolveNodeID(ctx, driveID, filePath)
	if err != nil {
		return PresignInfo{}, fmt.Errorf("presign download: %w", err)
	}
	n, err := s.Nodes.GetByID(ctx, nodeID)
	if err != nil {
		return PresignInfo{}, fmt.Errorf("presign download: load node: %w", err)
	}
	if !n.IsObject() {
		return PresignInfo{}, fmt.Errorf("presign download: %s is not an object", n.Type())
	}
	oc, err := n.ReadObject()
	if err != nil {
		return PresignInfo{}, err
	}
	url, err := s.Store.GetPresignedDownloadURL(ctx, oc.Bucket, oc.Key, expiry)
	if err != nil {
		return PresignInfo{}, fmt.Errorf("presign download: %w", err)
	}
	return PresignInfo{
		Method:    "GET",
		URL:       url,
		ExpiresAt: time.Now().Add(expiry),
	}, nil
}

// DeleteObject removes a single object from its bucket. Used by
// gc.UploadExpirer to clean up objects the client uploaded but
// never completed. Does not touch the node tree or the upload
// registry — callers handle those.
func (s *Service) DeleteObject(ctx context.Context, bucket, key string) error {
	return s.Store.DeleteObject(ctx, bucket, key)
}
