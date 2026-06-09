package object

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
	"github.com/mandacode-labs/retrowin-go/internal/logging"
)

// base64ToHex converts a base64-encoded string to hex-encoded string.
func base64ToHex(b64 string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(decoded), nil
}

// Provider represents the storage provider type.
type Provider string

const (
	ProviderS3 Provider = "s3"
)

// Status represents the object upload status.
type Status string

const (
	StatusPending Status = "pending"
	StatusActive  Status = "active"
)

// Object represents a tracked object in external storage.
type Object struct {
	id             string
	provider       Provider
	bucket         string
	systemID       string
	storageKey     string
	status         Status
	checksum       *string
	idempotencyKey *string
	createdAt      time.Time
	updatedAt      time.Time
}

// NewObject creates a new Object.
func NewObject(
	id string,
	provider Provider,
	bucket string,
	systemID string,
	storageKey string,
	status Status,
	checksum *string,
	idempotencyKey *string,
	createdAt time.Time,
	updatedAt time.Time,
) *Object {
	return &Object{
		id:             id,
		provider:       provider,
		bucket:         bucket,
		systemID:       systemID,
		storageKey:     storageKey,
		status:         status,
		checksum:       checksum,
		idempotencyKey: idempotencyKey,
		createdAt:      createdAt,
		updatedAt:      updatedAt,
	}
}

// Getters
func (o *Object) ID() string              { return o.id }
func (o *Object) Provider() Provider      { return o.provider }
func (o *Object) Bucket() string          { return o.bucket }
func (o *Object) SystemID() string        { return o.systemID }
func (o *Object) StorageKey() string      { return o.storageKey }
func (o *Object) Status() Status          { return o.status }
func (o *Object) Checksum() *string       { return o.checksum }
func (o *Object) IdempotencyKey() *string { return o.idempotencyKey }
func (o *Object) CreatedAt() time.Time    { return o.createdAt }
func (o *Object) UpdatedAt() time.Time    { return o.updatedAt }

// UploadSession contains information for client direct upload.
type UploadSession struct {
	ObjectID  string
	UploadURL string
	ExpiresAt time.Time
}

// CreateCommand for creating a new object (service layer).
type CreateCommand struct {
	Provider   Provider
	Bucket     string
	SystemID   string
	StorageKey string
	Reader     io.Reader
	Size       int64
}

// InitiateUploadCommand for starting a presigned upload.
type InitiateUploadCommand struct {
	SystemID       string
	ContentType    string
	Size           int64
	Checksum       *string // Base64-encoded MD5
	IdempotencyKey *string
}

// Filter for querying objects (service layer).
type Filter = QueryFilter

// Filter helpers
func ByID(id string) Filter {
	return Filter{ID: &id}
}

func ByIDs(ids []string) Filter {
	return Filter{IDs: ids}
}

func BySystemID(systemID string) Filter {
	return Filter{SystemID: &systemID}
}

func ByStatus(status Status) Filter {
	s := string(status)
	return Filter{Status: &s}
}

func BySystemIDAndStatus(systemID string, status Status) Filter {
	s := string(status)
	return Filter{SystemID: &systemID, Status: &s}
}

// Service implements object lifecycle operations.
type Service struct {
	repo    ObjectRepository
	storage Storage
}

// NewService creates a new Service.
func NewService(repo ObjectRepository, storage Storage) *Service {
	return &Service{repo: repo, storage: storage}
}

func (s *Service) Create(ctx context.Context, cmd *CreateCommand) (*Object, error) {
	if cmd.SystemID == "" {
		return nil, errors.BadRequest("system_id is required")
	}
	if cmd.StorageKey == "" {
		return nil, errors.BadRequest("storage_key is required")
	}

	provider := cmd.Provider
	if provider == "" {
		provider = ProviderS3
	}

	// Generate object ID
	objectID := uuid.New().String()

	// Stream upload to storage
	if err := s.storage.PutObject(ctx, cmd.Bucket, cmd.StorageKey, cmd.Reader, cmd.Size); err != nil {
		return nil, errors.WrapInternal(err, "failed to upload to storage")
	}

	// Create object record in DB
	params := &CreateParams{
		ID:         objectID,
		Provider:   provider,
		Bucket:     cmd.Bucket,
		SystemID:   cmd.SystemID,
		StorageKey: cmd.StorageKey,
	}
	obj, err := s.repo.Create(ctx, params)
	if err != nil {
		// Attempt cleanup on DB failure
		_ = s.storage.DeleteObject(ctx, cmd.Bucket, cmd.StorageKey)
		return nil, errors.WrapInternal(err, "failed to create object record")
	}

	return obj, nil
}

// InitiateUpload creates a pending object and returns presigned upload URL.
// If idempotencyKey is provided and a pending object already exists with the same key,
// the existing upload session is returned instead of creating a new one.
func (s *Service) InitiateUpload(ctx context.Context, cmd *InitiateUploadCommand) (*UploadSession, error) {
	if cmd.SystemID == "" {
		return nil, errors.BadRequest("system_id is required")
	}

	// Check idempotency: if a pending object exists with the same key, return it
	if cmd.IdempotencyKey != nil && *cmd.IdempotencyKey != "" {
		existing, err := s.repo.GetByIdempotencyKey(ctx, cmd.SystemID, *cmd.IdempotencyKey)
		if err != nil {
			return nil, errors.WrapInternal(err, "failed to check idempotency")
		}
		if existing != nil && existing.Status() == StatusPending {
			// Regenerate presigned URL (may have expired)
			expiry := ExpiryForSize(cmd.Size)
			checksum := ""
			if existing.Checksum() != nil {
				checksum = *existing.Checksum()
			}
			uploadURL, err := s.storage.GetPresignedUploadURL(ctx, existing.Bucket(), existing.StorageKey(), cmd.ContentType, cmd.Size, checksum, expiry)
			if err != nil {
				return nil, errors.WrapInternal(err, "failed to regenerate upload URL")
			}
			return &UploadSession{
				ObjectID:  existing.ID(),
				UploadURL: uploadURL,
				ExpiresAt: time.Now().Add(expiry),
			}, nil
		}
	}

	// Generate object ID and storage key
	objectID := uuid.New().String()
	storageKey := s.storage.KeyPrefix() + objectID

	// Resolve actual bucket name so it's stored explicitly in DB
	bucket := s.storage.DefaultBucket()

	// Build checksum string for storage
	var checksumStr *string
	if cmd.Checksum != nil && *cmd.Checksum != "" {
		checksumStr = cmd.Checksum
	}

	// Create pending object in DB
	params := &CreateParams{
		ID:             objectID,
		Provider:       ProviderS3,
		Bucket:         bucket,
		SystemID:       cmd.SystemID,
		StorageKey:     storageKey,
		Status:         StatusPending,
		Checksum:       checksumStr,
		IdempotencyKey: cmd.IdempotencyKey,
	}
	if _, err := s.repo.Create(ctx, params); err != nil {
		return nil, errors.WrapInternal(err, "failed to create pending object")
	}

	// Generate presigned upload URL with size-based expiry
	expiry := ExpiryForSize(cmd.Size)
	checksum := ""
	if checksumStr != nil {
		checksum = *checksumStr
	}
	uploadURL, err := s.storage.GetPresignedUploadURL(ctx, bucket, storageKey, cmd.ContentType, cmd.Size, checksum, expiry)
	if err != nil {
		// Cleanup on failure
		_ = s.repo.Delete(ctx, objectID)
		return nil, errors.WrapInternal(err, "failed to generate upload URL")
	}

	return &UploadSession{
		ObjectID:  objectID,
		UploadURL: uploadURL,
		ExpiresAt: time.Now().Add(expiry),
	}, nil
}

// CompleteUpload marks object as active after client confirms upload.
// Verifies the uploaded content matches the declared checksum (if provided).
func (s *Service) CompleteUpload(ctx context.Context, objectID string) (*Object, error) {
	obj, err := s.repo.GetByID(ctx, objectID)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.NotFound("object not found")
	}
	if obj.Status() != StatusPending {
		// Idempotent: if already active, return the existing object
		if obj.Status() == StatusActive {
			return obj, nil
		}
		return nil, errors.BadRequest("object is not in pending state")
	}

	// Verify object exists in storage
	exists, err := s.storage.ObjectExists(ctx, obj.Bucket(), obj.StorageKey())
	if err != nil {
		return nil, errors.WrapInternal(err, "failed to verify object")
	}
	if !exists {
		return nil, errors.BadRequest("object not found in storage")
	}

	// Verify checksum if provided
	if obj.Checksum() != nil && *obj.Checksum() != "" {
		actualChecksum, err := s.storage.GetObjectChecksum(ctx, obj.Bucket(), obj.StorageKey())
		if err != nil {
			return nil, errors.WrapInternal(err, "failed to verify object checksum")
		}
		// S3 ETag is hex-encoded MD5; stored checksum is base64-encoded MD5.
		// Convert base64 checksum to hex for comparison.
		expectedHex, err := base64ToHex(*obj.Checksum())
		if err != nil {
			return nil, errors.BadRequest(fmt.Sprintf("invalid checksum format: %v", err))
		}
		if actualChecksum != expectedHex {
			return nil, errors.BadRequest(fmt.Sprintf("checksum mismatch: expected %s, got %s", expectedHex, actualChecksum))
		}
	}

	// Update status to active
	if err := s.repo.UpdateStatus(ctx, objectID, StatusActive); err != nil {
		return nil, errors.WrapInternal(err, "failed to update object status")
	}

	return s.repo.GetByID(ctx, objectID)
}

func (s *Service) GetByID(ctx context.Context, id string) (*Object, error) {
	obj, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.NotFound("object not found")
	}
	return obj, nil
}

func (s *Service) GetByStorageKey(ctx context.Context, systemID string, provider Provider, bucket string, storageKey string) (*Object, error) {
	obj, err := s.repo.GetByStorageKey(ctx, systemID, string(provider), bucket, storageKey)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.NotFound("object not found")
	}
	return obj, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	obj, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if obj == nil {
		return errors.NotFound("object not found")
	}

	if err := s.storage.DeleteObject(ctx, obj.Bucket(), obj.StorageKey()); err != nil {
		return errors.WrapInternal(err, "failed to delete from storage")
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) DeleteBatch(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// Collect all objects
	objects := make([]*Object, 0, len(ids))
	for _, id := range ids {
		obj, err := s.repo.GetByID(ctx, id)
		if err != nil {
			logging.Ctx(ctx).Warn().
				Str("object_id", id).
				Err(err).
				Msg("failed to get object for batch delete, skipping")
			continue
		}
		if obj == nil {
			logging.Ctx(ctx).Warn().
				Str("object_id", id).
				Msg("object not found for batch delete, skipping")
			continue
		}
		objects = append(objects, obj)
	}

	if len(objects) == 0 {
		return nil, nil
	}

	// Group by bucket
	bucketKeys := make(map[string][]string)
	for _, obj := range objects {
		bucketKeys[obj.Bucket()] = append(bucketKeys[obj.Bucket()], obj.StorageKey())
	}

	// Batch delete from S3
	for bucket, keys := range bucketKeys {
		if err := s.storage.DeleteObjects(ctx, bucket, keys); err != nil {
			logging.Ctx(ctx).Error().
				Str("bucket", bucket).
				Int("key_count", len(keys)).
				Err(err).
				Msg("failed to batch delete objects from storage")
			// Continue with DB cleanup anyway
		}
	}

	// Delete from DB
	deleted := make([]string, 0, len(objects))
	for _, obj := range objects {
		if err := s.repo.Delete(ctx, obj.ID()); err != nil {
			logging.Ctx(ctx).Warn().
				Str("object_id", obj.ID()).
				Err(err).
				Msg("failed to delete object record from DB, skipping")
			continue
		}
		deleted = append(deleted, obj.ID())
	}

	return deleted, nil
}

func (s *Service) DeleteBySystemID(ctx context.Context, systemID string) error {
	return s.repo.DeleteBySystemID(ctx, systemID)
}

func (s *Service) CleanupStorageBySystemID(ctx context.Context, systemID string) error {
	objects, err := s.repo.Find(ctx, &QueryFilter{SystemID: &systemID})
	if err != nil {
		return err
	}
	for _, obj := range objects {
		if err := s.storage.DeleteObject(ctx, obj.Bucket(), obj.StorageKey()); err != nil {
			logging.Ctx(ctx).Warn().
				Str("object_id", obj.ID()).
				Err(err).
				Msg("failed to delete object from storage, skipping")
		}
	}
	return nil
}

func (s *Service) Find(ctx context.Context, filter Filter) ([]*Object, error) {
	return s.repo.Find(ctx, &filter)
}

func (s *Service) FindOne(ctx context.Context, filter Filter) (*Object, error) {
	obj, err := s.repo.FindOne(ctx, &filter)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.NotFound("object not found")
	}
	return obj, nil
}

func (s *Service) GetDownloadURL(ctx context.Context, id string, size int64) (string, time.Time, error) {
	obj, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", time.Time{}, err
	}
	if obj == nil {
		return "", time.Time{}, errors.NotFound("object not found")
	}

	expiry := ExpiryForSize(size)
	downloadURL, err := s.storage.GetPresignedDownloadURL(ctx, obj.Bucket(), obj.StorageKey(), expiry)
	if err != nil {
		return "", time.Time{}, errors.WrapInternal(err, "failed to generate download URL")
	}
	return downloadURL, time.Now().Add(expiry), nil
}

// FindPendingOlderThan finds pending objects older than threshold for GC.
func (s *Service) FindPendingOlderThan(ctx context.Context, olderThan time.Duration) ([]*Object, error) {
	return s.repo.FindPendingOlderThan(ctx, olderThan)
}

// FindActive finds all active objects for GC orphan detection.
func (s *Service) FindActive(ctx context.Context) ([]*Object, error) {
	return s.repo.FindActive(ctx)
}

// DeleteFromDB removes the object record from DB only (no S3 call).
func (s *Service) DeleteFromDB(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// GetObjectSize returns the size of the object in external storage.
func (s *Service) GetObjectSize(ctx context.Context, id string) (int64, error) {
	obj, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return 0, err
	}
	if obj == nil {
		return 0, errors.NotFound("object not found")
	}

	size, err := s.storage.GetObjectSize(ctx, obj.Bucket(), obj.StorageKey())
	if err != nil {
		return 0, errors.WrapInternal(err, "failed to get object size from storage")
	}
	return size, nil
}
