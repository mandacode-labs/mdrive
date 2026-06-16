// Package gc implements the garbage collection job for mdrive.
//
// Unlike apiserver, gc is a job — it runs periodically (or once) and
// performs cleanup work. It is intentionally separate from the API
// server so that GC load doesn't affect user-facing request latency.
package gc

import (
	"context"
	"fmt"
	"time"

	"github.com/mandacode-labs/mdrive/internal/app"
	"github.com/mandacode-labs/mdrive/internal/logging"
)

// Runner runs the GC job loop.
type Runner struct {
	app    *app.App
	log    *logging.Logger
	tick   time.Duration
}

// Config for the GC Runner.
type Config struct {
	// Tick is the interval between GC cycles. Zero means run once and exit.
	Tick time.Duration
}

// NewRunner creates a new GC Runner.
func NewRunner(a *app.App, cfg Config) *Runner {
	return &Runner{
		app:  a,
		log:  a.Log,
		tick: cfg.Tick,
	}
}

// Run starts the GC job.
//
// If Tick is zero, runs a single cycle and returns.
// Otherwise, runs cycles periodically until ctx is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	if r.tick <= 0 {
		// Single-shot mode
		return r.runOnce(ctx)
	}

	r.log.Info().Dur("tick", r.tick).Msg("gc: starting periodic loop")
	ticker := time.NewTicker(r.tick)
	defer ticker.Stop()

	// Run once immediately, then on each tick.
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

// runOnce performs a single GC cycle.
//
// Future work: scan nodes + S3 to find orphans.
// For now, this is a stub that returns nil.
func (r *Runner) runOnce(ctx context.Context) error {
	r.log.Info().Msg("gc: cycle starting")
	_ = ctx
	// TODO: implement orphan detection
	//  1. List S3 bucket objects
	//  2. Compare against nodes table
	//  3. Delete orphan S3 objects
	r.log.Info().Msg("gc: cycle complete")
	return nil
}

// Run is a top-level convenience that constructs and runs the GC Runner.
func Run(ctx context.Context, a *app.App, cfg Config) error {
	if err := NewRunner(a, cfg).Run(ctx); err != nil {
		return fmt.Errorf("gc: %w", err)
	}
	return nil
}
