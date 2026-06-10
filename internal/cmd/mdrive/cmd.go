// Package mdrive implements the mdrive command
package mdrive

import (
	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/cmd/gc"
	"github.com/mandacode-labs/mdrive/internal/cmd/migrate"
	"github.com/mandacode-labs/mdrive/internal/cmd/serve"
)

// NewCmd creates a new cobra command for mdrive
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mdrive",
		Short: "Mandacode Drive API server",
	}

	// Add subcommands from separate packages
	cmd.AddCommand(serve.NewCmd())
	cmd.AddCommand(migrate.NewCmd())
	cmd.AddCommand(gc.NewCmd())

	return cmd
}
