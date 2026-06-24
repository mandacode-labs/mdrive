// Package gc implements garbage collection background jobs.
package gc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/internal/auth/session"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/upload/s3"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

const defaultProcessLimit = 1000

// Runner is the common interface for a GC job that runs to
// completion when invoked. Each subcommand of the `gc` CLI
// produces a Runner from the per-job constructor (NewTombstoneCleaner,
// NewDrivePurger, NewUploadExpirer, NewSessionExpirer). The runners
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

func (c *TombstoneCleaner) Run(ctx context.Context) error {
	c.log.Info("gc: tombstones starting")

	groups, err := QueryTombstones(ctx, c.client, defaultProcessLimit)
	if err != nil {
		return fmt.Errorf("gc: query tombstones: %w", err)
	}
	if len(groups) == 0 {
		c.log.Info("gc: tombstones complete (no tombstones)")
		return nil
	}

	for _, g := range groups {
		if err := c.processGroup(ctx, g); err != nil {
			c.log.Error("gc: bucket cleanup failed", "err", err, "bucket", g.Bucket, "count", len(g.Keys))
			continue
		}
		if err := DeleteTombstones(ctx, c.client, g.IDs); err != nil {
			c.log.Error("gc: delete tombstones failed", "err", err, "bucket", g.Bucket)
		}
	}

	c.log.Info("gc: tombstones complete", "groups", len(groups))
	return nil
}

func (c *TombstoneCleaner) processGroup(ctx context.Context, g TombstoneGroup) error {
	c.log.Info("gc: deleting S3 objects", "bucket", g.Bucket, "keys", len(g.Keys))

	storage, err := FindStorageByBucket(ctx, c.client, g.Bucket)
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
// objects. It scans the upload registry for tokens whose ExpiresAt has
// passed, deletes the S3 object (best-effort, tombstone on failure), and
// removes the registry entry. Safe to run on registries that do not
// implement Scanner (logs a warning and returns).
type UploadExpirer struct {
	reg     upload.Registry
	upload  *upload.Service
	garbage *GarbageRecorder
	log     *slog.Logger
}

func NewUploadExpirer(reg upload.Registry, uploadSvc *upload.Service, garbage *GarbageRecorder, log *slog.Logger) *UploadExpirer {
	return &UploadExpirer{reg: reg, upload: uploadSvc, garbage: garbage, log: log}
}

func (e *UploadExpirer) Run(ctx context.Context) error {
	e.log.Info("gc: expire-uploads starting")
	defer e.log.Info("gc: expire-uploads complete")

	scanner, ok := e.reg.(interface {
		Scan(ctx context.Context, fn func(id string) error) error
	})
	if !ok {
		e.log.Warn("gc: upload registry does not support Scan; skipping")
		return nil
	}

	var scanned, deleted, s3err int
	err := scanner.Scan(ctx, func(id string) error {
		scanned++
		meta, err := e.reg.Get(ctx, id)
		if err != nil {
			_ = e.reg.Delete(ctx, id)
			return nil
		}
		if !meta.IsExpired() {
			return nil
		}
		bucket := meta.Bucket
		key := meta.Key
		if e.upload != nil && bucket != "" && key != "" {
			if err := e.upload.DeleteObject(ctx, bucket, key); err != nil {
				s3err++
				e.log.Warn("gc: delete upload object failed", "err", err, "bucket", bucket, "key", key)
				if e.garbage != nil {
					_ = e.garbage.RecordGarbage(ctx, []vfs.GarbageRef{{Bucket: bucket, Key: key}})
				}
			}
		}
		if err := e.reg.Delete(ctx, id); err != nil {
			e.log.Warn("gc: delete upload token failed", "err", err, "upload_id", id)
			return nil
		}
		deleted++
		return nil
	})
	if err != nil {
		return fmt.Errorf("gc: upload scan: %w", err)
	}
	e.log.Info("gc: uploads", "scanned", scanned, "deleted", deleted, "s3_errors", s3err)
	return nil
}

// SessionExpirer removes expired sessions from the session store. It scans
// the store (when supported) and deletes any session whose ExpiresAt has
// passed. Backends that do not implement session.Scanner are skipped with
// a warning.
type SessionExpirer struct {
	store session.Store
	log   *slog.Logger
}

func NewSessionExpirer(store session.Store, log *slog.Logger) *SessionExpirer {
	return &SessionExpirer{store: store, log: log}
}

func (e *SessionExpirer) Run(ctx context.Context) error {
	e.log.Info("gc: expire-sessions starting")
	defer e.log.Info("gc: expire-sessions complete")

	if e.store == nil {
		e.log.Warn("gc: no session store; skipping")
		return nil
	}
	scanner, ok := e.store.(session.Scanner)
	if !ok {
		e.log.Warn("gc: session store does not support Scan; skipping")
		return nil
	}
	var scanned, deleted int
	err := scanner.Scan(ctx, func(id string) error {
		scanned++
		sess, err := e.store.Get(ctx, id)
		if err != nil {
			_ = e.store.Delete(ctx, id)
			deleted++
			return nil
		}
		if sess.IsExpired() {
			if err := e.store.Delete(ctx, id); err == nil {
				deleted++
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("gc: session scan: %w", err)
	}
	e.log.Info("gc: sessions", "scanned", scanned, "deleted", deleted)
	return nil
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
