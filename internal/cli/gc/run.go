// Package gc implements the `mdrive gc` subcommand.
package gc

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/app"
	gcjobs "github.com/mandacode-labs/mdrive/internal/app/gc"
	"github.com/mandacode-labs/mdrive/internal/config"
)

// NewCmd creates the `gc` subcommand.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Run garbage collection jobs",
	}
	cmd.AddCommand(newTombstonesCmd())
	cmd.AddCommand(newPurgeDrivesCmd())
	cmd.AddCommand(newExpireUploadsCmd())
	cmd.AddCommand(newExpireSessionsCmd())
	cmd.AddCommand(newRunAllCmd())
	return cmd
}

func newTombstonesCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "tombstones",
		Short: "Delete S3 objects recorded in gc_tombstones",
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
			return gcjobs.NewTombstoneCleaner(a).Run(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	return cmd
}

func newPurgeDrivesCmd() *cobra.Command {
	var (
		configPath string
		retention  time.Duration
	)
	cmd := &cobra.Command{
		Use:   "purge-drives",
		Short: "Permanently remove soft-deleted drives older than the retention period",
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
			return gcjobs.NewDrivePurger(a, retention).Run(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	cmd.Flags().DurationVar(&retention, "retention", 0, "minimum age of deleted drives to purge (default 168h)")
	return cmd
}

func newExpireUploadsCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "expire-uploads",
		Short: "Remove stale upload registrations",
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
			return gcjobs.NewUploadExpirer(a).Run(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	return cmd
}

func newExpireSessionsCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "expire-sessions",
		Short: "Remove expired sessions",
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
			return gcjobs.NewSessionExpirer(a).Run(cmd.Context())
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	return cmd
}

func newRunAllCmd() *cobra.Command {
	var (
		configPath string
		tick       time.Duration
	)
	cmd := &cobra.Command{
		Use:   "run-all",
		Short: "Run all GC jobs (one-shot by default, periodic with --tick)",
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
			return gcjobs.Run(cmd.Context(), a, gcjobs.Config{Tick: tick})
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	cmd.Flags().DurationVar(&tick, "tick", 0, "if set (>0), run periodically at this interval instead of one-shot")
	return cmd
}
