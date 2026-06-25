package cliflags

import "github.com/spf13/cobra"

// AddConfigFlag registers the standard --config/-c flag on cmd
// and binds it to *configPath. Centralized so the default and
// help text don't drift across subcommands.
func AddConfigFlag(cmd *cobra.Command, configPath *string) {
	cmd.Flags().StringVarP(configPath, "config", "c", "config.yaml", "path to config file")
}