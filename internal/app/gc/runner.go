// Package gc implements garbage collection background jobs.
package gc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mandacode-labs/mdrive/internal/app"
	"github.com/mandacode-labs/mdrive/internal/auth/session"
	"github.com/mandacode-labs/mdrive/internal/storage/s3"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

const defaultProcessLimit = 1000

// Runner is the common interface for a GC job that runs to
// completion when invoked. Each subcommand of the `gc` CLI
// produces a Runner from a *app.App.
type Runner interface {
	Run(ctx context.Context) error
}

// TombstoneCleaner deletes S3 objects recorded in gc_tombstones.
type TombstoneCleaner struct {
	app *app.App
	log *slog.Logger
}

func NewTombstoneCleaner(a *app.App) *TombstoneCleaner {
	return &TombstoneCleaner{app: a, log: a.Log}
}

func (c *TombstoneCleaner) Run(ctx context.Context) error {
	c.log.Info("gc: tombstones starting")

	groups, err := app.QueryTombstones(ctx, c.app.Ent, defaultProcessLimit)
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
		if err := app.DeleteTombstones(ctx, c.app.Ent, g.IDs); err != nil {
			c.log.Error("gc: delete tombstones failed", "err", err, "bucket", g.Bucket)
		}
	}

	c.log.Info("gc: tombstones complete", "groups", len(groups))
	return nil
}

func (c *TombstoneCleaner) processGroup(ctx context.Context, g app.TombstoneGroup) error {
	c.log.Info("gc: deleting S3 objects", "bucket", g.Bucket, "keys", len(g.Keys))

	cfg, err := app.FindStorageByBucket(ctx, c.app.Ent, g.Bucket)
	if err != nil {
		return fmt.Errorf("gc: find storage: %w", err)
	}
	client, err := s3.NewClient(ctx, cfg)
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
	app       *app.App
	log       *slog.Logger
	retention time.Duration
}

func NewDrivePurger(a *app.App, retention time.Duration) *DrivePurger {
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	return &DrivePurger{app: a, log: a.Log, retention: retention}
}

func (p *DrivePurger) Run(ctx context.Context) error {
	p.log.Info("gc: purge-drives starting", "retention", p.retention)
	before := time.Now().Add(-p.retention)
	drives, err := p.app.DriveSvc.ListDeletedForAdmin(ctx, true, before, defaultProcessLimit)
	if err != nil {
		return fmt.Errorf("gc: list deleted drives: %w", err)
	}
	for _, d := range drives {
		if err := p.app.DriveSvc.Purge(ctx, d.ID()); err != nil {
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
	app *app.App
	log *slog.Logger
}

func NewUploadExpirer(a *app.App) *UploadExpirer {
	return &UploadExpirer{app: a, log: a.Log}
}

func (e *UploadExpirer) Run(ctx context.Context) error {
	e.log.Info("gc: expire-uploads starting")
	defer e.log.Info("gc: expire-uploads complete")

	scanner, ok := e.app.UploadReg.(upload.Scanner)
	if !ok {
		e.log.Warn("gc: upload registry does not support Scan; skipping")
		return nil
	}

	var scanned, deleted, s3err int
	err := scanner.Scan(ctx, func(id string) error {
		scanned++
		meta, err := e.app.UploadReg.Get(ctx, id)
		if err != nil {
			// Token already gone or expired: just clean up the registry entry.
			_ = e.app.UploadReg.Delete(ctx, id)
			return nil
		}
		if !meta.IsExpired() {
			return nil
		}
		// Best-effort: delete the S3 object the client may have uploaded
		// but never completed. On failure, record a tombstone so the
		// tombstone-cleaner job can retry.
		bucket := meta.Bucket
		key := meta.Key
		if e.app.Store != nil && bucket != "" && key != "" {
			if err := e.app.Store.DeleteObject(ctx, bucket, key); err != nil {
				s3err++
				e.log.Warn("gc: delete upload object failed", "err", err, "bucket", bucket, "key", key)
				if e.app.TombstoneInserter != nil {
					_ = e.app.TombstoneInserter.InsertTombstones(ctx, []vfs.ObjectRef{{Bucket: bucket, Key: key}})
				}
			}
		}
		if err := e.app.UploadReg.Delete(ctx, id); err != nil {
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
	app *app.App
	log *slog.Logger
}

func NewSessionExpirer(a *app.App) *SessionExpirer {
	return &SessionExpirer{app: a, log: a.Log}
}

func (e *SessionExpirer) Run(ctx context.Context) error {
	e.log.Info("gc: expire-sessions starting")
	defer e.log.Info("gc: expire-sessions complete")

	if e.app.SessionStore == nil {
		e.log.Warn("gc: no session store; skipping")
		return nil
	}
	scanner, ok := e.app.SessionStore.(session.Scanner)
	if !ok {
		e.log.Warn("gc: session store does not support Scan; skipping")
		return nil
	}
	var scanned, deleted int
	err := scanner.Scan(ctx, func(id string) error {
		scanned++
		sess, err := e.app.SessionStore.Get(ctx, id)
		if err != nil {
			_ = e.app.SessionStore.Delete(ctx, id)
			deleted++
			return nil
		}
		if sess.IsExpired() {
			if err := e.app.SessionStore.Delete(ctx, id); err == nil {
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
