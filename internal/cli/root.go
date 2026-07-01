package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/cli/apiserver"
	"github.com/mandacode-labs/mdrive/internal/cli/gc"
	"github.com/mandacode-labs/mdrive/internal/cli/migrate"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

var Version = "dev"

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "mdrive",
		Short:         "mdrive: POSIX-like filesystem over S3",
		Long:          "mdrive provides a multi-tenant, POSIX-like filesystem API backed by S3-compatible object storage.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(apiserver.NewCmd())
	root.AddCommand(gc.NewCmd())
	root.AddCommand(migrate.NewCmd())
	return root
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		chain := errorx.Chain(err)
		if chain == "" {
			fmt.Fprintf(os.Stderr, "mdrive: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "mdrive: %s\n", chain)
		}
		os.Exit(1)
	}
}
