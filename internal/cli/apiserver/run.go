package apiserver

import (
	"github.com/spf13/cobra"

	server "github.com/mandacode-labs/mdrive/internal/app/apiserver"
	"github.com/mandacode-labs/mdrive/internal/app"
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
			fs := vfs.NewService(a.NodeSvc, a.DriveSvc, a.UserSvc, nil, a.Perm, a.UploadReg, a.TombstoneInserter)
			return server.NewServer(a, fs).Run()
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	return cmd
}
