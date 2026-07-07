// Package gc implements garbage collection background jobs.
package gc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
	"github.com/mandacode-labs/mdrive/internal/logx"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/upload/s3"
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
}

func NewTombstoneCleaner(client *ent.Client) *TombstoneCleaner {
	return &TombstoneCleaner{client: client}
}

func (t *TombstoneCleaner) Run(ctx context.Context) error {
	logx.Info(ctx, "gc.tombstones.starting")

	groups, err := QueryTombstones(ctx, t.client, defaultProcessLimit)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("gc: tombstone query (limit=%d)", defaultProcessLimit))
	}
	if len(groups) == 0 {
		logx.Info(ctx, "gc.tombstones.complete",
			slog.Int("groups", 0),
		)
		return nil
	}

	for _, g := range groups {
		if err := t.processGroup(ctx, g); err != nil {
			logx.Warn(ctx, "gc.bucket_cleanup.failed",
				slog.String("err", err.Error()),
				slog.String("bucket", g.Bucket),
				slog.Int("count", len(g.Keys)),
			)
			continue
		}
		if err := DeleteTombstones(ctx, t.client, g.IDs); err != nil {
			logx.Warn(ctx, "gc.delete_tombstones.failed",
				slog.String("err", err.Error()),
				slog.String("bucket", g.Bucket),
			)
		}
	}

	logx.Info(ctx, "gc.tombstones.complete",
		slog.Int("groups", len(groups)),
	)
	return nil
}

func (t *TombstoneCleaner) processGroup(ctx context.Context, g TombstoneGroup) error {
	logx.Info(ctx, "gc.deleting_s3_objects",
		slog.String("bucket", g.Bucket),
		slog.Int("keys", len(g.Keys)),
	)

	storage, err := FindStorageByBucket(ctx, t.client, g.Bucket)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("gc: tombstone find storage (bucket=%s)", g.Bucket))
	}
	client, err := s3.NewClient(ctx, s3.Config{
		Region:       storage.Region,
		Endpoint:     stringPtr(storage.Endpoint),
		AccessKey:    storage.AccessKey,
		SecretKey:    storage.SecretKey,
		UsePathStyle: storage.UsePathStyle,
	})
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("gc: tombstone s3 client (bucket=%s, region=%s)", g.Bucket, storage.Region))
	}
	if err := client.DeleteObjects(ctx, g.Bucket, g.Keys); err != nil {
		return errorx.Wrap(err, fmt.Sprintf("gc: tombstone delete objects (bucket=%s, count=%d)", g.Bucket, len(g.Keys)))
	}
	return nil
}

// DrivePurger permanently removes drives that have been soft-deleted longer than retention.
type DrivePurger struct {
	driveSvc  drive.Service
	retention time.Duration
}

func NewDrivePurger(driveSvc drive.Service, retention time.Duration) *DrivePurger {
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	return &DrivePurger{driveSvc: driveSvc, retention: retention}
}

func (p *DrivePurger) Run(ctx context.Context) error {
	logx.Info(ctx, "gc.purge_drives.starting",
		slog.Duration("retention", p.retention),
	)
	before := time.Now().Add(-p.retention)
	drives, err := p.driveSvc.ListDeletedForAdmin(ctx, true, before, defaultProcessLimit)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("gc: list deleted drives (before=%s, limit=%d)", before.Format(time.RFC3339), defaultProcessLimit))
	}
	for _, d := range drives {
		if err := p.driveSvc.Purge(ctx, d.ID()); err != nil {
			logx.Warn(ctx, "gc.purge_drive.failed",
				slog.String("err", err.Error()),
				slog.String("drive_id", d.ID()),
			)
			continue
		}
		logx.Info(ctx, "gc.drive.purged",
			slog.String("drive_id", d.ID()),
		)
	}
	logx.Info(ctx, "gc.purge_drives.complete",
		slog.Int("count", len(drives)),
	)
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
	uploadService upload.Service
	garbage       *Recorder
}

func NewUploadExpirer(reg upload.TokenRegistry, uploadSvc upload.Service, garbage *Recorder) *UploadExpirer {
	return &UploadExpirer{tokenRegistry: reg, uploadService: uploadSvc, garbage: garbage}
}

func (u *UploadExpirer) Run(ctx context.Context) error {
	logx.Info(ctx, "gc.expire_uploads.starting")
	defer func() {
		logx.Info(ctx, "gc.expire_uploads.complete")
	}()

	scanner, ok := u.tokenRegistry.(upload.TokenScanner)
	if !ok {
		logx.Warn(ctx, "gc.upload_registry.unsupported")
		return nil
	}

	var scanned, deleted, s3err int
	err := scanner.Scan(ctx, func(id string) error {
		scanned++
		meta, err := u.tokenRegistry.Get(ctx, id)
		if err != nil {
			logx.Warn(ctx, "gc.get_upload_token.failed",
				slog.String("err", err.Error()),
				slog.String("upload_id", id),
			)
			if derr := u.tokenRegistry.Delete(ctx, id); derr != nil {
				logx.Warn(ctx, "gc.delete_orphan_token.failed",
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
				logx.Warn(ctx, "gc.delete_upload_object.failed",
					slog.String("err", err.Error()),
					slog.String("bucket", bucket),
					slog.String("key", key),
				)
				if u.garbage != nil {
					_ = u.garbage.RecordGarbage(ctx, []vfs.GarbageRef{{Bucket: bucket, Key: key}})
				}
			}
		}
		if err := u.tokenRegistry.Delete(ctx, id); err != nil {
			logx.Warn(ctx, "gc.delete_upload_token.failed",
				slog.String("err", err.Error()),
				slog.String("upload_id", id),
			)
			return nil
		}
		deleted++
		return nil
	})
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("gc: upload scan (scanned=%d, deleted=%d, s3_errors=%d)", scanned, deleted, s3err))
	}
	logx.Info(ctx, "gc.uploads.summary",
		slog.Int("scanned", scanned),
		slog.Int("deleted", deleted),
		slog.Int("s3_errors", s3err),
	)
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
