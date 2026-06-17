package gc

import (
	"context"
	"fmt"
	"time"

	"github.com/mandacode-labs/mdrive/internal/app"
	"github.com/mandacode-labs/mdrive/internal/logging"
	"github.com/mandacode-labs/mdrive/internal/storage/s3"
)

type Runner struct {
	app  *app.App
	log  *logging.Logger
	tick time.Duration
}

type Config struct {
	Tick time.Duration
}

func NewRunner(a *app.App, cfg Config) *Runner {
	return &Runner{
		app:  a,
		log:  a.Log,
		tick: cfg.Tick,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	if r.tick <= 0 {
		return r.runOnce(ctx)
	}

	r.log.Info().Dur("tick", r.tick).Msg("gc: starting periodic loop")
	ticker := time.NewTicker(r.tick)
	defer ticker.Stop()

	if err := r.runOnce(ctx); err != nil {
		r.log.Error().Err(err).Msg("gc: cycle failed")
	}
	for {
		select {
		case <-ctx.Done():
			r.log.Info().Msg("gc: stopping")
			return nil
		case <-ticker.C:
			if err := r.runOnce(ctx); err != nil {
				r.log.Error().Err(err).Msg("gc: cycle failed")
			}
		}
	}
}

const defaultProcessLimit = 1000

func (r *Runner) runOnce(ctx context.Context) error {
	r.log.Info().Msg("gc: cycle starting")

	groups, err := app.QueryTombstones(ctx, r.app.Ent, defaultProcessLimit)
	if err != nil {
		return fmt.Errorf("gc: query tombstones: %w", err)
	}
	if len(groups) == 0 {
		r.log.Info().Msg("gc: cycle complete (no tombstones)")
		return nil
	}

	for _, g := range groups {
		if err := r.processGroup(ctx, g); err != nil {
			r.log.Error().Err(err).Str("bucket", g.Bucket).Int("count", len(g.Keys)).Msg("gc: bucket cleanup failed")
			continue
		}
		if err := app.DeleteTombstones(ctx, r.app.Ent, g.IDs); err != nil {
			r.log.Error().Err(err).Str("bucket", g.Bucket).Msg("gc: delete tombstones failed")
		}
	}

	r.log.Info().Int("groups", len(groups)).Msg("gc: cycle complete")
	return nil
}

func (r *Runner) processGroup(ctx context.Context, g app.TombstoneGroup) error {
	r.log.Info().Str("bucket", g.Bucket).Int("keys", len(g.Keys)).Msg("gc: deleting S3 objects")

	cfg, err := app.FindStorageByBucket(ctx, r.app.Ent, g.Bucket)
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

func Run(ctx context.Context, a *app.App, cfg Config) error {
	if err := NewRunner(a, cfg).Run(ctx); err != nil {
		return fmt.Errorf("gc: %w", err)
	}
	return nil
}
