package apiserver

import "github.com/spf13/cobra"

// NewCmd returns the api-server subcommand.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api-server",
		Short: "Manage the HTTP API server",
	}
	cmd.AddCommand(newRunCmd())
	return cmd
}
