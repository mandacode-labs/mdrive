package gc

import (
	"github.com/mandacode-labs/mdrive/internal/app"
	gcjobs "github.com/mandacode-labs/mdrive/internal/app/gc"
	"github.com/spf13/cobra"
)

// NewCmd returns the gc subcommand.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Run garbage collection jobs",
	}
	addJob(cmd, "tombstones", "Delete S3 objects recorded in gc_tombstones",
		func(a *app.App) gcjobs.Runner { return gcjobs.NewTombstoneCleaner(a.Ent) })
	addJob(cmd, "purge-drives", "Permanently remove soft-deleted drives older than the retention period",
		func(a *app.App) gcjobs.Runner { return gcjobs.NewDrivePurger(a.DriveSvc, 0) }, "retention", "0s", "minimum age of deleted drives to purge")
	addJob(cmd, "expire-uploads", "Remove stale upload registrations",
		func(a *app.App) gcjobs.Runner {
			return gcjobs.NewUploadExpirer(a.UploadToken, a.UploadSvc, a.Garbage)
		})
	return cmd
}
