// Package gc implements the `mdrive gc` subcommand.
package gc

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/app"
	gcjobs "github.com/mandacode-labs/mdrive/internal/app/gc"
	"github.com/mandacode-labs/mdrive/internal/cliflags"
	"github.com/mandacode-labs/mdrive/internal/config"
)

// NewCmd creates the `gc` subcommand.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Run garbage collection jobs",
	}
	addJob(cmd, "tombstones", "Delete S3 objects recorded in gc_tombstones",
		func(a *app.App) gcjobs.Runner { return gcjobs.NewTombstoneCleaner(a.Ent, a.Log) })
	addJob(cmd, "purge-drives", "Permanently remove soft-deleted drives older than the retention period",
		func(a *app.App) gcjobs.Runner { return gcjobs.NewDrivePurger(a.DriveSvc, a.Log, 0) }, "retention", "0s", "minimum age of deleted drives to purge")
	addJob(cmd, "expire-uploads", "Remove stale upload registrations",
		func(a *app.App) gcjobs.Runner {
			return gcjobs.NewUploadExpirer(a.UploadToken, a.UploadSvc, a.Garbage, a.Log)
		})
	addJob(cmd, "expire-sessions", "Remove expired sessions",
		func(a *app.App) gcjobs.Runner { return gcjobs.NewSessionExpirer(a.SessionStore, a.Log) })
	return cmd
}

// addJob wires a subcommand with the standard config/app lifecycle.
// The factory is called once the app is built, so failures during
// app construction (config load, db connect, etc.) surface before
// the job starts.
//
// Extra flagSpecs (name, default, usage) configure additional
// job-specific flags beyond --config.
func addJob(parent *cobra.Command, use, short string, factory func(*app.App) gcjobs.Runner, flagSpecs ...string) {
	var configPath string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadFromPath(configPath)
			if err != nil {
				return err
			}
			a, err := app.New(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			defer func() { _ = a.Close() }()
			return factory(a).Run(cmd.Context())
		},
	}
	cliflags.AddConfigFlag(cmd, &configPath)
	for i := 0; i+2 < len(flagSpecs); i += 3 {
		flag, def, usage := flagSpecs[i], flagSpecs[i+1], flagSpecs[i+2]
		cmd.Flags().Duration(flag, parseDurationOrZero(def), usage)
	}
	parent.AddCommand(cmd)
}

func parseDurationOrZero(s string) time.Duration {
	if s == "" || s == "0" || s == "0s" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}
