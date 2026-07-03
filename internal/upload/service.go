package upload

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"time"

	"github.com/google/uuid"

	coredrive "github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
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

// StorageLookup is the data-access contract for the storage
// config of a drive. The underlying *drive.Service satisfies it
// via GetStorage.
type StorageLookup interface {
	GetStorage(ctx context.Context, driveID string) (*coredrive.Storage, error)
}

// NodeLifecycle is the subset of node.Service the upload flow
// needs: create object node, link into parent, delete on
// failure, and look up an existing node by ID.
type NodeLifecycle interface {
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
// upload token registry with the vfs primitives (node tree, path
// resolution) and the S3 store.
//
// Permission checks are the caller's responsibility: the handler
// must call Authorizer.Check on the drive before invoking
// any of the three methods below.
type Service struct {
	TokenRegistry TokenRegistry
	StorageLookup StorageLookup
	NodeLifecycle NodeLifecycle
	ObjectStore   ObjectStore
	Path          PathResolver
	tm            TxManager
}

// TxManager runs a function inside a transaction.
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Config groups Service dependencies.
type Config struct {
	TokenRegistry TokenRegistry
	StorageLookup StorageLookup
	NodeLifecycle NodeLifecycle
	ObjectStore   ObjectStore
	Path          PathResolver
	TxManager     TxManager
}

// NewService wires a Service. A nil TokenRegistry defaults to a
// MemoryRegistry (in-process, lazy TTL expiry on Get); production
// code should pass a Valkey-backed registry.
func NewService(cfg Config) *Service {
	if cfg.TokenRegistry == nil {
		cfg.TokenRegistry = NewMemoryRegistry()
	}
	return &Service{
		TokenRegistry: cfg.TokenRegistry,
		StorageLookup: cfg.StorageLookup,
		NodeLifecycle: cfg.NodeLifecycle,
		ObjectStore:   cfg.ObjectStore,
		Path:          cfg.Path,
		tm:            cfg.TxManager,
	}
}

// InitiateUpload creates a presigned PUT URL for uploading an object directly to S3.
// The returned uploadID must be passed to CompleteUpload after the client PUTs the bytes.
// Permission is the caller's responsibility.
func (s *Service) InitiateUpload(ctx context.Context, userID, driveID, destPath string, contentType *string, contentLength *int64, expiry time.Duration) (PresignInfo, error) {
	_ = userID
	storage, err := s.StorageLookup.GetStorage(ctx, driveID)
	if err != nil {
		return PresignInfo{}, errorx.Wrap(err, fmt.Sprintf("upload: initiate storage lookup (drive_id=%s)", driveID))
	}
	bucket := storage.Bucket()
	if bucket == "" {
		return PresignInfo{}, errorx.New(errorx.KindBadRequest, "upload: drive has no bucket configured (drive_id="+driveID+")")
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
	if err := s.TokenRegistry.Put(ctx, meta, expiry); err != nil {
		return PresignInfo{}, errorx.Wrap(err, fmt.Sprintf("upload: initiate registry put (drive_id=%s, upload_id=%s)", driveID, uploadID))
	}

	url, err := s.ObjectStore.GetPresignedUploadURL(ctx, bucket, key, expiry)
	if err != nil {
		_ = s.TokenRegistry.Delete(ctx, uploadID)
		return PresignInfo{}, errorx.Wrap(err, fmt.Sprintf("upload: initiate presign (drive_id=%s, upload_id=%s)", driveID, uploadID))
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
//
// userID is the principal initiating the completion; it must match
// the userID stored on the upload token. Mismatch returns
// ErrUploadOwnershipMismatch — the upload ID is not a bearer
// credential across users.
func (s *Service) CompleteUpload(ctx context.Context, userID, driveID, uploadID string, contentLength int64, checksum *string) (*node.Node, error) {
	meta, err := s.TokenRegistry.Get(ctx, uploadID)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("upload: complete token get (upload_id=%s)", uploadID))
	}
	if meta.UserID != userID {
		return nil, errorx.New(errorx.KindForbidden, "upload: token does not match user")
	}
	if meta.DriveID != driveID {
		return nil, errorx.New(errorx.KindBadRequest, "upload: token does not match drive")
	}
	if meta.Size != nil && *meta.Size != contentLength {
		return nil, errorx.New(errorx.KindBadRequest, "upload: complete size mismatch (expected="+strconv.FormatInt(*meta.Size, 10)+", got="+strconv.FormatInt(contentLength, 10)+")")
	}

	exists, err := s.ObjectStore.ObjectExists(ctx, meta.Bucket, meta.Key)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("upload: complete object exists check (bucket=%s, key=%s)", meta.Bucket, meta.Key))
	}
	if !exists {
		return nil, errorx.New(errorx.KindNotFound, "upload: S3 object was not uploaded")
	}

	var n *node.Node
	if err := s.tm.WithTx(ctx, func(ctx context.Context) error {
		parentID, name, err := s.Path.ResolveParentNodeID(ctx, driveID, meta.Path)
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("upload: complete resolve parent (path=%s)", meta.Path))
		}
		parent, err := s.NodeLifecycle.GetByID(ctx, parentID)
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("upload: complete load parent (parent_id=%s)", parentID))
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
		created, err := s.NodeLifecycle.CreateObject(ctx, obj, contentLength)
		if err != nil {
			return err
		}
		if lerr := s.NodeLifecycle.Link(ctx, parent, name, created); lerr != nil {
			if derr := s.NodeLifecycle.Delete(ctx, created.ID()); derr != nil {
				return errorx.Wrap(lerr, fmt.Sprintf("upload: complete link (node_id=%s, cleanup_err=%v)", created.ID(), derr))
			}
			return errorx.Wrap(lerr, fmt.Sprintf("upload: complete link (node_id=%s)", created.ID()))
		}
		n = created
		return nil
	}); err != nil {
		return nil, err
	}

	if err := s.TokenRegistry.Delete(ctx, uploadID); err != nil {
		return n, errorx.Wrap(err, fmt.Sprintf("upload: complete cleanup token (upload_id=%s)", uploadID))
	}
	return n, nil
}

// PresignDownload returns a presigned GET URL for an existing object node.
// Permission is the caller's responsibility.
func (s *Service) PresignDownload(ctx context.Context, userID, driveID, filePath string, expiry time.Duration) (PresignInfo, error) {
	_ = userID
	nodeID, err := s.Path.ResolveNodeID(ctx, driveID, filePath)
	if err != nil {
		return PresignInfo{}, errorx.Wrap(err, fmt.Sprintf("upload: presign download resolve (path=%s)", filePath))
	}
	n, err := s.NodeLifecycle.GetByID(ctx, nodeID)
	if err != nil {
		return PresignInfo{}, errorx.Wrap(err, fmt.Sprintf("upload: presign download load node (node_id=%s)", nodeID))
	}
	if n == nil {
		return PresignInfo{}, errorx.New(errorx.KindNotFound, fmt.Sprintf("upload: presign download node not found (node_id=%s)", nodeID))
	}
	if !n.IsObject() {
		return PresignInfo{}, errorx.New(errorx.KindBadRequest, "upload: presign download target is not an object (type="+string(n.Kind())+")")
	}
	oc, err := n.ReadObject()
	if err != nil {
		return PresignInfo{}, errorx.Wrap(err, fmt.Sprintf("upload: presign download read object (node_id=%s)", nodeID))
	}
	url, err := s.ObjectStore.GetPresignedDownloadURL(ctx, oc.Bucket, oc.Key, expiry)
	if err != nil {
		return PresignInfo{}, errorx.Wrap(err, fmt.Sprintf("upload: presign download s3 url (bucket=%s, key=%s)", oc.Bucket, oc.Key))
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
	if err := s.ObjectStore.DeleteObject(ctx, bucket, key); err != nil {
		return errorx.Wrap(err, fmt.Sprintf("upload: delete object (bucket=%s, key=%s)", bucket, key))
	}
	return nil
}
