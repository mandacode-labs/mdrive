// Package gc implements garbage collection background jobs.
package gc

import (
	"context"
	"fmt"
	"time"

	"github.com/mandacode-labs/mdrive/internal/app"
	"github.com/mandacode-labs/mdrive/internal/auth/session"
	"github.com/mandacode-labs/mdrive/internal/logging"
	"github.com/mandacode-labs/mdrive/internal/storage/s3"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

// vfsObjectRef aliases vfs.ObjectRef to keep the GC logic readable.
type vfsObjectRef = vfs.ObjectRef

const defaultProcessLimit = 1000

// TombstoneCleaner deletes S3 objects recorded in gc_tombstones.
type TombstoneCleaner struct {
	app *app.App
	log *logging.Logger
}

func NewTombstoneCleaner(a *app.App) *TombstoneCleaner {
	return &TombstoneCleaner{app: a, log: a.Log}
}

func (c *TombstoneCleaner) Run(ctx context.Context) error {
	c.log.Info().Msg("gc: tombstones starting")

	groups, err := app.QueryTombstones(ctx, c.app.Ent, defaultProcessLimit)
	if err != nil {
		return fmt.Errorf("gc: query tombstones: %w", err)
	}
	if len(groups) == 0 {
		c.log.Info().Msg("gc: tombstones complete (no tombstones)")
		return nil
	}

	for _, g := range groups {
		if err := c.processGroup(ctx, g); err != nil {
			c.log.Error().Err(err).Str("bucket", g.Bucket).Int("count", len(g.Keys)).Msg("gc: bucket cleanup failed")
			continue
		}
		if err := app.DeleteTombstones(ctx, c.app.Ent, g.IDs); err != nil {
			c.log.Error().Err(err).Str("bucket", g.Bucket).Msg("gc: delete tombstones failed")
		}
	}

	c.log.Info().Int("groups", len(groups)).Msg("gc: tombstones complete")
	return nil
}

func (c *TombstoneCleaner) processGroup(ctx context.Context, g app.TombstoneGroup) error {
	c.log.Info().Str("bucket", g.Bucket).Int("keys", len(g.Keys)).Msg("gc: deleting S3 objects")

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
	log       *logging.Logger
	retention time.Duration
}

func NewDrivePurger(a *app.App, retention time.Duration) *DrivePurger {
	if retention <= 0 {
		retention = 7 * 24 * time.Hour
	}
	return &DrivePurger{app: a, log: a.Log, retention: retention}
}

func (p *DrivePurger) Run(ctx context.Context) error {
	p.log.Info().Dur("retention", p.retention).Msg("gc: purge-drives starting")
	before := time.Now().Add(-p.retention)
	drives, err := p.app.DriveSvc.ListDeleted(ctx, before, defaultProcessLimit)
	if err != nil {
		return fmt.Errorf("gc: list deleted drives: %w", err)
	}
	for _, d := range drives {
		if err := p.app.DriveSvc.Purge(ctx, d.ID()); err != nil {
			p.log.Error().Err(err).Str("drive_id", d.ID()).Msg("gc: purge drive failed")
			continue
		}
		p.log.Info().Str("drive_id", d.ID()).Msg("gc: purged drive")
	}
	p.log.Info().Int("count", len(drives)).Msg("gc: purge-drives complete")
	return nil
}

// UploadExpirer removes stale upload registrations and their backing S3
// objects. It scans the upload registry for tokens whose ExpiresAt has
// passed, deletes the S3 object (best-effort, tombstone on failure), and
// removes the registry entry. Safe to run on registries that do not
// implement Scanner (logs a warning and returns).
type UploadExpirer struct {
	app *app.App
	log *logging.Logger
}

func NewUploadExpirer(a *app.App) *UploadExpirer {
	return &UploadExpirer{app: a, log: a.Log}
}

func (e *UploadExpirer) Run(ctx context.Context) error {
	e.log.Info().Msg("gc: expire-uploads starting")
	defer e.log.Info().Msg("gc: expire-uploads complete")

	scanner, ok := e.app.UploadReg.(upload.Scanner)
	if !ok {
		e.log.Warn().Msg("gc: upload registry does not support Scan; skipping")
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
				e.log.Warn().Err(err).Str("bucket", bucket).Str("key", key).Msg("gc: delete upload object failed")
				if e.app.TombstoneInserter != nil {
					_ = e.app.TombstoneInserter.InsertTombstones(ctx, []vfsObjectRef{{Bucket: bucket, Key: key}})
				}
			}
		}
		if err := e.app.UploadReg.Delete(ctx, id); err != nil {
			e.log.Warn().Err(err).Str("upload_id", id).Msg("gc: delete upload token failed")
			return nil
		}
		deleted++
		return nil
	})
	if err != nil {
		return fmt.Errorf("gc: upload scan: %w", err)
	}
	e.log.Info().Int("scanned", scanned).Int("deleted", deleted).Int("s3_errors", s3err).Msg("gc: uploads")
	return nil
}

// SessionExpirer removes expired sessions from the session store. It scans
// the store (when supported) and deletes any session whose ExpiresAt has
// passed. Backends that do not implement session.Scanner are skipped with
// a warning.
type SessionExpirer struct {
	app *app.App
	log *logging.Logger
}

func NewSessionExpirer(a *app.App) *SessionExpirer {
	return &SessionExpirer{app: a, log: a.Log}
}

func (e *SessionExpirer) Run(ctx context.Context) error {
	e.log.Info().Msg("gc: expire-sessions starting")
	defer e.log.Info().Msg("gc: expire-sessions complete")

	if e.app.SessionStore == nil {
		e.log.Warn().Msg("gc: no session store; skipping")
		return nil
	}
	scanner, ok := e.app.SessionStore.(session.Scanner)
	if !ok {
		e.log.Warn().Msg("gc: session store does not support Scan; skipping")
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
	e.log.Info().Int("scanned", scanned).Int("deleted", deleted).Msg("gc: sessions")
	return nil
}
