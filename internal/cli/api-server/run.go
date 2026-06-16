package apiservercli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/app"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/vfs"
)

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
		Short: "Run the API server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.LoadFromPath(configPath)
			if err != nil {
				return err
			}
			a, err := app.New(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			fs := vfs.NewService(a.NodeSvc, a.DriveSvc, a.UserSvc, nil, nil)
			return apiserver.NewServer(a, fs, placeholderUser).Run()
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	return cmd
}

func placeholderUser(_ context.Context) (string, bool) {
	return "default", true
}
