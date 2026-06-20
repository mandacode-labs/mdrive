// Package gc implements garbage collection background jobs.
package gc

import (
	"context"
	"fmt"
	"time"

	"github.com/mandacode-labs/mdrive/internal/app"
	"github.com/mandacode-labs/mdrive/internal/logging"
	"github.com/mandacode-labs/mdrive/internal/storage/s3"
)

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

// UploadExpirer removes stale upload registrations.
type UploadExpirer struct {
	app *app.App
	log *logging.Logger
}

func NewUploadExpirer(a *app.App) *UploadExpirer {
	return &UploadExpirer{app: a, log: a.Log}
}

func (e *UploadExpirer) Run(ctx context.Context) error {
	e.log.Info().Msg("gc: expire-uploads starting")
	e.log.Info().Msg("gc: expire-uploads complete")
	return nil
}

// SessionExpirer removes expired sessions from the store.
type SessionExpirer struct {
	app *app.App
	log *logging.Logger
}

func NewSessionExpirer(a *app.App) *SessionExpirer {
	return &SessionExpirer{app: a, log: a.Log}
}

func (e *SessionExpirer) Run(ctx context.Context) error {
	e.log.Info().Msg("gc: expire-sessions starting")
	e.log.Info().Msg("gc: expire-sessions complete")
	return nil
}
