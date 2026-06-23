package apiserver

import (
	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/app"
	server "github.com/mandacode-labs/mdrive/internal/app/apiserver"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/drive"
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
			fs := vfs.NewService(vfs.ServiceConfig{
				Node:  a.NodeSvc,
				Drive: a.DriveSvc,
				User:  a.UserSvc,
				Store: a.Store,
				Perm:  a.Perm,
				Reg:   a.UploadReg,
				GC:    a.TombstoneInserter,
			})
			driveSvc := drive.NewService(drive.Config{
				Drive: a.DriveSvc,
				Perm:  a.Perm,
			})
			return server.NewServer(a, fs, driveSvc).Run()
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	return cmd
}
