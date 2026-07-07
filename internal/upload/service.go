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
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
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

// Service is the vfs-level upload orchestrator. It composes the
// upload token registry with the vfs primitives (node tree, path
// resolution) and the S3 store.
//
// Permission checks are the caller's responsibility: the handler
// must call Authorizer.Check on the drive before invoking
// any of the methods below.
//
// Callers depend on this single interface; the unexported service
// struct is the only implementation.
type Service interface {
	InitiateUpload(ctx context.Context, userID, driveID, destPath string, contentType *string, contentLength *int64, expiry time.Duration) (PresignInfo, error)
	CompleteUpload(ctx context.Context, userID, driveID, uploadID string, contentLength int64, checksum *string) (*node.Node, error)
	PresignDownload(ctx context.Context, userID, driveID, filePath string, expiry time.Duration) (PresignInfo, error)
	DeleteObject(ctx context.Context, bucket, key string) error
}

// service is the only implementation of Service.
type service struct {
	TokenRegistry TokenRegistry
	StorageLookup coredrive.Service
	NodeLifecycle node.NodeOperation
	ObjectStore   ObjectStore
	Path          fs.Service
	tm            entx.TxManager
}

// Config groups the dependencies of NewService.
type Config struct {
	TokenRegistry TokenRegistry
	StorageLookup coredrive.Service
	NodeLifecycle node.NodeOperation
	ObjectStore   ObjectStore
	Path          fs.Service
	TxManager     entx.TxManager
}

// NewService wires a service. A nil TokenRegistry defaults to a
// MemoryRegistry (in-process, lazy TTL expiry on Get); production
// code should pass a Valkey-backed registry.
func NewService(cfg Config) Service {
	if cfg.TokenRegistry == nil {
		cfg.TokenRegistry = NewMemoryRegistry()
	}
	return &service{
		TokenRegistry: cfg.TokenRegistry,
		StorageLookup: cfg.StorageLookup,
		NodeLifecycle: cfg.NodeLifecycle,
		ObjectStore:   cfg.ObjectStore,
		Path:          cfg.Path,
		tm:            cfg.TxManager,
	}
}

var _ Service = (*service)(nil)

func (s *service) InitiateUpload(ctx context.Context, userID, driveID, destPath string, contentType *string, contentLength *int64, expiry time.Duration) (PresignInfo, error) {
	_ = userID
	storage, err := s.StorageLookup.GetStorage(ctx, driveID)
	if err != nil {
		return PresignInfo{}, errorx.Wrap(err, fmt.Sprintf("upload: initiate storage lookup (drive_id=%s)", driveID))
	}
	bucket := storage.Bucket()
	if bucket == "" {
		return PresignInfo{}, errorx.New(errorx.KindFailedPrecondition, "upload: drive has no bucket configured (drive_id="+driveID+")")
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

func (s *service) CompleteUpload(ctx context.Context, userID, driveID, uploadID string, contentLength int64, checksum *string) (*node.Node, error) {
	meta, err := s.TokenRegistry.Get(ctx, uploadID)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("upload: complete token get (upload_id=%s)", uploadID))
	}
	if meta.UserID != userID {
		return nil, errorx.New(errorx.KindPermissionDenied, "upload: token does not match user")
	}
	if meta.DriveID != driveID {
		return nil, errorx.New(errorx.KindInvalidArgument, "upload: token does not match drive")
	}
	if meta.Size != nil && *meta.Size != contentLength {
		return nil, errorx.New(errorx.KindInvalidArgument, "upload: complete size mismatch (expected="+strconv.FormatInt(*meta.Size, 10)+", got="+strconv.FormatInt(contentLength, 10)+")")
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
		dirPath := path.Dir(meta.Path)
		name := path.Base(meta.Path)
		parentDentry, err := s.Path.Walk(ctx, driveID, dirPath)
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("upload: complete resolve parent (path=%s)", meta.Path))
		}
		parent, err := s.NodeLifecycle.GetByID(ctx, parentDentry.Node.ID())
		if err != nil {
			return errorx.Wrap(err, fmt.Sprintf("upload: complete load parent (parent_id=%s)", parentDentry.Node.ID()))
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

func (s *service) PresignDownload(ctx context.Context, userID, driveID, filePath string, expiry time.Duration) (PresignInfo, error) {
	_ = userID
	dentry, err := s.Path.Walk(ctx, driveID, filePath)
	if err != nil {
		return PresignInfo{}, errorx.Wrap(err, fmt.Sprintf("upload: presign download resolve (path=%s)", filePath))
	}
	n, err := s.NodeLifecycle.GetByID(ctx, denty.Node.ID())
	if err != nil {
		return PresignInfo{}, errorx.Wrap(err, fmt.Sprintf("upload: presign download load node (node_id=%s)", denty.Node.ID()))
	}
	if n == nil {
		return PresignInfo{}, errorx.New(errorx.KindNotFound, fmt.Sprintf("upload: presign download node not found (node_id=%s)", denty.Node.ID()))
	}
	if !n.IsObject() {
		return PresignInfo{}, errorx.New(errorx.KindInvalidArgument, "upload: presign download target is not an object (type="+string(n.Kind())+")")
	}
	oc, err := n.ReadObject()
	if err != nil {
		return PresignInfo{}, errorx.Wrap(err, fmt.Sprintf("upload: presign download read object (node_id=%s)", denty.Node.ID()))
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

func (s *service) DeleteObject(ctx context.Context, bucket, key string) error {
	if err := s.ObjectStore.DeleteObject(ctx, bucket, key); err != nil {
		return errorx.Wrap(err, fmt.Sprintf("upload: delete object (bucket=%s, key=%s)", bucket, key))
	}
	return nil
}
