package storage

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/mandacode-labs/retrowin-go/internal/application/fs"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
	"github.com/mandacode-labs/retrowin-go/internal/core/object"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

// StorageService defines the interface for file storage operations.
type StorageService interface {
	// InitiateUpload creates a pending object and returns presigned upload URL.
	InitiateUpload(ctx context.Context, cmd *InitiateUploadCommand) (*object.UploadSession, error)

	// CompleteUpload verifies upload completion and creates the inode.
	CompleteUpload(ctx context.Context, cmd *CompleteUploadCommand) (*UploadResult, error)

	GetDownloadURL(ctx context.Context, id string) (string, time.Time, error)

	// DeleteObjectByInode deletes the S3 object referenced by the given inode.
	// Parses ObjectContent from inode content, then deletes the object from storage and DB.
	DeleteObjectByInode(ctx context.Context, inodeID string) error
}

// InitiateUploadCommand for starting a presigned upload.
type InitiateUploadCommand struct {
	SystemID       string
	ContentType    string
	Size           int64
	Checksum       *string // Base64-encoded MD5
	IdempotencyKey *string
}

// CompleteUploadCommand for finalizing upload after client confirms.
type CompleteUploadCommand struct {
	ObjectID string
	SystemID string
	Mode     int
	Flags    int
}

// UploadResult contains the created inode and object.
type UploadResult struct {
	Inode  *inode.Inode
	Object *object.Object
}

type service struct {
	fsSvc     fs.FsService
	objectSvc object.ObjectService
}

// NewService creates a new storage service.
func NewService(fsSvc fs.FsService, objectSvc object.ObjectService) StorageService {
	return &service{
		fsSvc:     fsSvc,
		objectSvc: objectSvc,
	}
}

// InitiateUpload creates a pending object and returns presigned upload URL.
func (s *service) InitiateUpload(ctx context.Context, cmd *InitiateUploadCommand) (*object.UploadSession, error) {
	if cmd.SystemID == "" {
		return nil, errors.BadRequest("system_id is required")
	}

	session, err := s.objectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
		SystemID:       cmd.SystemID,
		ContentType:    cmd.ContentType,
		Size:           cmd.Size,
		Checksum:       cmd.Checksum,
		IdempotencyKey: cmd.IdempotencyKey,
	})
	if err != nil {
		return nil, errors.WrapInternal(err, "failed to initiate upload")
	}

	return session, nil
}

// CompleteUpload verifies upload and creates inode.
func (s *service) CompleteUpload(ctx context.Context, cmd *CompleteUploadCommand) (*UploadResult, error) {
	if cmd.ObjectID == "" {
		return nil, errors.BadRequest("object_id is required")
	}
	if cmd.SystemID == "" {
		return nil, errors.BadRequest("system_id is required")
	}

	// Mark object as active (verifies storage existence internally)
	obj, err := s.objectSvc.CompleteUpload(ctx, cmd.ObjectID)
	if err != nil {
		return nil, errors.FromError(err)
	}

	// Get object size from storage
	size, err := s.objectSvc.GetObjectSize(ctx, obj.ID())
	if err != nil {
		return nil, errors.FromError(err)
	}

	// Create inode with object reference
	return s.createInodeWithObject(ctx, cmd.SystemID, cmd.Mode, cmd.Flags, obj.ID(), size)
}

// createInodeWithObject creates an inode referencing the given object.
func (s *service) createInodeWithObject(ctx context.Context, systemID string, mode int, flags int, objectID string, size int64) (*UploadResult, error) {
	// Store ObjectContent in inode content
	c := &content.ObjectContent{ObjectID: objectID}
	cBytes, err := json.Marshal(c)
	if err != nil {
		return nil, errors.WrapInternal(err, "failed to marshal object content")
	}

	// Set default mode if not provided
	if mode == 0 {
		mode = inode.ModeObject | inode.PermOwnerRW | inode.PermGroupRX | inode.PermOtherR
	}

	createdInode, err := s.fsSvc.CreateFile(ctx, &fs.CreateFileCommand{
		SystemID: systemID,
		Mode:     mode,
		Size:     size,
		Flags:    flags,
		Content:  cBytes,
	})
	if err != nil {
		return nil, errors.WrapInternal(err, "failed to create inode")
	}

	// Get the object for the result
	obj, err := s.objectSvc.GetByID(ctx, objectID)
	if err != nil {
		obj = nil // Object not critical for result
	}

	return &UploadResult{
		Inode:  createdInode,
		Object: obj,
	}, nil
}

func (s *service) GetDownloadURL(ctx context.Context, id string) (string, time.Time, error) {
	in, err := s.fsSvc.Get(ctx, id)
	if err != nil {
		return "", time.Time{}, err
	}

	var c content.ObjectContent
	if err := json.Unmarshal(in.Content(), &c); err != nil {
		return "", time.Time{}, errors.WrapInternal(err, "failed to parse object content")
	}
	if c.ObjectID == "" {
		return "", time.Time{}, errors.BadRequest("inode has no object reference")
	}

	return s.objectSvc.GetDownloadURL(ctx, c.ObjectID, in.Size())
}

// DeleteObjectByInode deletes the S3 object referenced by the given inode.
func (s *service) DeleteObjectByInode(ctx context.Context, inodeID string) error {
	in, err := s.fsSvc.Get(ctx, inodeID)
	if err != nil {
		return err
	}

	// Only process object inodes
	if in.Mode()&inode.ModeTypeMask != inode.ModeObject {
		return nil
	}

	var c content.ObjectContent
	if err := json.Unmarshal(in.Content(), &c); err != nil {
		return nil // Not an object content, skip silently
	}
	if c.ObjectID == "" {
		return nil
	}

	// Best-effort S3 cleanup - ignore storage errors
	if err := s.objectSvc.Delete(ctx, c.ObjectID); err != nil {
		slog.Warn("failed to delete object from storage, skipping",
			"object_id", c.ObjectID,
			"error", err,
		)
	}
	return nil
}
