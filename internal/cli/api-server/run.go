// Package apiservercli implements the `mdrive api-server` subcommand.
package apiservercli

import (
	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/app"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver"
	"github.com/mandacode-labs/mdrive/internal/config"
)

// NewCmd creates the `api-server` subcommand.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api-server",
		Short: "Manage the HTTP API server",
	}
	cmd.AddCommand(newRunCmd())
	return cmd
}

func newRunCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the API server (long-running HTTP service)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadFromPath(configPath)
			if err != nil {
				return err
			}
			a, err := app.New(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			return apiserver.NewServer(a).Run()
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	return cmd
}
