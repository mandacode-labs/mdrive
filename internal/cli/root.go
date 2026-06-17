package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	apisvr "github.com/mandacode-labs/mdrive/internal/cli/apisvr"
	"github.com/mandacode-labs/mdrive/internal/cli/gc"
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
	root.AddCommand(apisvr.NewCmd())
	root.AddCommand(gc.NewCmd())
	return root
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "mdrive: %v\n", err)
		os.Exit(1)
	}
}
