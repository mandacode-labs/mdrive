package migrate

import "github.com/spf13/cobra"

// NewCmd returns the migrate subcommand.
func NewCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database migrations",
	}
	root.AddCommand(newApplyCmd())
	return root
}
