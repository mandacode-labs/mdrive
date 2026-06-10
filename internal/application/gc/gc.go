package gc

import (
	"context"
	"time"

	"github.com/mandacode-labs/mdrive/internal/core/object"
	"github.com/mandacode-labs/mdrive/internal/logging"
)

const (
	// DefaultPendingExpiry is the default time after which pending objects are considered expired.
	DefaultPendingExpiry = 24 * time.Hour
)

// GarbageCollector handles cleanup of expired pending objects and orphaned DB records.
type GarbageCollector struct {
	objectSvc ObjectService
	storage   object.Storage
	expiry    time.Duration
}

// NewGarbageCollector creates a new garbage collector.
func NewGarbageCollector(objectSvc ObjectService, storage object.Storage, expiry time.Duration) *GarbageCollector {
	if expiry == 0 {
		expiry = DefaultPendingExpiry
	}
	return &GarbageCollector{
		objectSvc: objectSvc,
		storage:   storage,
		expiry:    expiry,
	}
}

// GCResult holds the results of a garbage collection run.
type GCResult struct {
	PendingCleaned int
	OrphansCleaned int
}

// Run performs a full garbage collection pass.
func (gc *GarbageCollector) Run(ctx context.Context) (*GCResult, error) {
	result := &GCResult{}

	// Clean expired pending objects
	pendingCleaned, err := gc.cleanupPending(ctx)
	if err != nil {
		return result, err
	}
	result.PendingCleaned = pendingCleaned

	// Clean orphaned DB records (active objects missing from S3)
	orphanCleaned, err := gc.cleanupOrphans(ctx)
	if err != nil {
		return result, err
	}
	result.OrphansCleaned = orphanCleaned

	return result, nil
}

// cleanupPending removes expired pending objects from storage and DB.
func (gc *GarbageCollector) cleanupPending(ctx context.Context) (int, error) {
	pendingObjects, err := gc.objectSvc.FindPendingOlderThan(ctx, gc.expiry)
	if err != nil {
		return 0, err
	}

	cleaned := 0
	for _, obj := range pendingObjects {
		if err := gc.objectSvc.Delete(ctx, obj.ID()); err != nil {
			logging.Ctx(ctx).Warn().
				Str("object_id", obj.ID()).
				Err(err).
				Msg("failed to delete expired pending object")
			continue
		}
		cleaned++
		logging.Ctx(ctx).Info().
			Str("object_id", obj.ID()).
			Dur("age", time.Since(obj.CreatedAt())).
			Msg("cleaned up expired pending object")
	}

	return cleaned, nil
}

// cleanupOrphans finds active objects missing from S3 and removes their DB records.
func (gc *GarbageCollector) cleanupOrphans(ctx context.Context) (int, error) {
	activeObjects, err := gc.objectSvc.FindActive(ctx)
	if err != nil {
		return 0, err
	}

	cleaned := 0
	for _, obj := range activeObjects {
		exists, err := gc.storage.ObjectExists(ctx, obj.Bucket(), obj.StorageKey())
		if err != nil {
			logging.Ctx(ctx).Warn().
				Str("object_id", obj.ID()).
				Err(err).
				Msg("failed to check object existence, skipping")
			continue
		}
		if exists {
			continue
		}

		// S3 data is gone — clean up DB record only
		if err := gc.objectSvc.DeleteFromDB(ctx, obj.ID()); err != nil {
			logging.Ctx(ctx).Warn().
				Str("object_id", obj.ID()).
				Err(err).
				Msg("failed to delete orphaned object from DB")
			continue
		}
		cleaned++
		logging.Ctx(ctx).Info().
			Str("object_id", obj.ID()).
			Str("bucket", obj.Bucket()).
			Str("storage_key", obj.StorageKey()).
			Msg("cleaned up orphaned object")
	}

	return cleaned, nil
}
