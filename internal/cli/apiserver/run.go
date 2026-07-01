package apiserver

import (
	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/app"
	server "github.com/mandacode-labs/mdrive/internal/app/apiserver"
	"github.com/mandacode-labs/mdrive/internal/cliflags"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/errorx"
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
			srv, err := server.NewServer(a, a.VFS, a.DriveSvc, a.UploadSvc, a.UserSvc, a.Authorizer)
			if err != nil {
				return errorx.Wrap(err, "apiserver: new server")
			}
			return srv.Run()
		},
	}
	cliflags.AddConfigFlag(cmd, &configPath)
	return cmd
}