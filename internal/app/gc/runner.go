// Package gc implements garbage collection background jobs.
package gc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/upload/s3"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

const defaultProcessLimit = 1000

// Runner is the common interface for a GC job that runs to
// completion when invoked. Each subcommand of the `gc` CLI
// produces a Runner from the per-job constructor (NewTombstoneCleaner,
// NewDrivePurger, NewUploadExpirer). The runners
// depend only on the specific services they need — no shared
// "app context" struct — so the gc package can stay out of the
// app.App type's import cycle.
type Runner interface {
	Run(ctx context.Context) error
}

// TombstoneCleaner drains gc_tombstones: for each bucket group it
// looks up the S3 credentials of a drive using that bucket, calls
// S3 DeleteObjects, and removes the tombstone rows on success.
// This job is the only consumer of internal/upload/s3 in production;
// vfs and upload.Service both route their garbage through the
// GarbageRecorder (the DB writer), not through the S3 client
// directly.
type TombstoneCleaner struct {
	client *ent.Client
	log    *slog.Logger
}

func NewTombstoneCleaner(client *ent.Client, log *slog.Logger) *TombstoneCleaner {
	return &TombstoneCleaner{client: client, log: log}
}

func (t *TombstoneCleaner) Run(ctx context.Context) error {
	t.log.Info("gc: tombstones starting")

	groups, err := QueryTombstones(ctx, t.client, defaultProcessLimit)
	if err != nil {
		return fmt.Errorf("gc: query tombstones: %w", err)
	}
	if len(groups) == 0 {
		t.log.Info("gc: tombstones complete (no tombstones)")
		return nil
	}

	for _, g := range groups {
		if err := t.processGroup(ctx, g); err != nil {
			t.log.Error("gc: bucket cleanup failed", "err", err, "bucket", g.Bucket, "count", len(g.Keys))
			continue
		}
		if err := DeleteTombstones(ctx, t.client, g.IDs); err != nil {
			t.log.Error("gc: delete tombstones failed", "err", err, "bucket", g.Bucket)
		}
	}

	t.log.Info("gc: tombstones complete", "groups", len(groups))
	return nil
}

func (t *TombstoneCleaner) processGroup(ctx context.Context, g TombstoneGroup) error {
	t.log.Info("gc: deleting S3 objects", "bucket", g.Bucket, "keys", len(g.Keys))

	storage, err := FindStorageByBucket(ctx, t.client, g.Bucket)
	if err != nil {
		return fmt.Errorf("gc: find storage: %w", err)
	}
	client, err := s3.NewClient(ctx, s3.Config{
		Region:       storage.Region,
		Endpoint:     stringPtr(storage.Endpoint),
		AccessKey:    storage.AccessKey,
		SecretKey:    storage.SecretKey,
		UsePathStyle: storage.UsePathStyle,
	})
	if err != nil {
		return fmt.Errorf("gc: s3 client: %w", err)
	}
	if err := client.DeleteObjects(ctx, g.Bucket, g.Keys); err != nil {
		return fmt.Errorf("gc: delete objects: %w", err)
	}
	return nil
}

// DrivePurger permanently removes drives that have been soft-deleted longer than retention.
type DrivePurger struct {
	driveSvc  *drive.Service
	log       *slog.Logger
	retention time.Duration
}

func NewDrivePurger(driveSvc *drive.Service, log *slog.Logger, retention time.Duration) *DrivePurger {
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	return &DrivePurger{driveSvc: driveSvc, log: log, retention: retention}
}

func (p *DrivePurger) Run(ctx context.Context) error {
	p.log.Info("gc: purge-drives starting", "retention", p.retention)
	before := time.Now().Add(-p.retention)
	drives, err := p.driveSvc.ListDeletedForAdmin(ctx, true, before, defaultProcessLimit)
	if err != nil {
		return fmt.Errorf("gc: list deleted drives: %w", err)
	}
	for _, d := range drives {
		if err := p.driveSvc.Purge(ctx, d.ID()); err != nil {
			p.log.Error("gc: purge drive failed", "err", err, "drive_id", d.ID())
			continue
		}
		p.log.Info("gc: purged drive", "drive_id", d.ID())
	}
	p.log.Info("gc: purge-drives complete", "count", len(drives))
	return nil
}

// UploadExpirer removes stale upload registrations and their backing S3
// objects. It scans the upload token registry for tokens whose
// ExpiresAt has passed, deletes the S3 object (best-effort, tombstone
// on failure), and removes the registry entry. Safe to run on
// registries that do not implement TokenScanner (logs a warning and
// returns).
type UploadExpirer struct {
	tokenRegistry upload.TokenRegistry
	uploadService *upload.Service
	garbage       *GarbageRecorder
	log           *slog.Logger
}

func NewUploadExpirer(reg upload.TokenRegistry, uploadSvc *upload.Service, garbage *GarbageRecorder, log *slog.Logger) *UploadExpirer {
	return &UploadExpirer{tokenRegistry: reg, uploadService: uploadSvc, garbage: garbage, log: log}
}

func (u *UploadExpirer) Run(ctx context.Context) error {
	u.log.Info("gc: expire-uploads starting")
	defer u.log.Info("gc: expire-uploads complete")

	scanner, ok := u.tokenRegistry.(upload.TokenScanner)
	if !ok {
		u.log.Warn("gc: upload registry does not support Scan; skipping")
		return nil
	}

	var scanned, deleted, s3err int
	err := scanner.Scan(ctx, func(id string) error {
		scanned++
		meta, err := u.tokenRegistry.Get(ctx, id)
		if err != nil {
			u.log.Warn("gc: get upload token failed (treating as orphan)",
				slog.String("err", err.Error()),
				slog.String("upload_id", id),
			)
			if derr := u.tokenRegistry.Delete(ctx, id); derr != nil {
				u.log.Warn("gc: delete orphan upload token failed",
					slog.String("err", derr.Error()),
					slog.String("upload_id", id),
				)
			}
			return nil
		}
		if !meta.IsExpired() {
			return nil
		}
		bucket := meta.Bucket
		key := meta.Key
		if u.uploadService != nil && bucket != "" && key != "" {
			if err := u.uploadService.DeleteObject(ctx, bucket, key); err != nil {
				s3err++
				u.log.Warn("gc: delete upload object failed", "err", err, "bucket", bucket, "key", key)
				if u.garbage != nil {
					_ = u.garbage.RecordGarbage(ctx, []vfs.GarbageRef{{Bucket: bucket, Key: key}})
				}
			}
		}
		if err := u.tokenRegistry.Delete(ctx, id); err != nil {
			u.log.Warn("gc: delete upload token failed", "err", err, "upload_id", id)
			return nil
		}
		deleted++
		return nil
	})
	if err != nil {
		return fmt.Errorf("gc: upload scan: %w", err)
	}
	u.log.Info("gc: uploads", "scanned", scanned, "deleted", deleted, "s3_errors", s3err)
	return nil
}

// SessionExpirer was removed when the encrypted cookie session
// replaced the server-side session store. The OIDC cookie session
// has its own TTL; no GC needed.

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
