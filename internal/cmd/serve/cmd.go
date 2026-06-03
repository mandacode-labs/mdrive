// Package serve implements the serve command
package serve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	openapispec "github.com/mandacode-labs/retrowin-go/api"
)

// NewCmd creates a new cobra command for the serve command
func NewCmd() *cobra.Command {
	var cfgFile string
	var port int
	var openAPIPath string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the API server",
		Run: func(cmd *cobra.Command, args []string) {
			var spec []byte
			var err error
			if openAPIPath != "" {
				spec, err = readOpenAPIFile(openAPIPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error reading OpenAPI spec: %v\n", err)
					os.Exit(1)
				}
			} else {
				spec = openapispec.Spec
			}
			app := NewFXApp(cfgFile, port, spec)
			app.Run()
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file path")
	cmd.Flags().IntVarP(&port, "port", "p", 8080, "server port")
	cmd.Flags().StringVar(&openAPIPath, "openapi", "", "OpenAPI spec file path (defaults to embedded spec)")

	return cmd
}

func readOpenAPIFile(path string) ([]byte, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	clean := filepath.Clean(abs)
	if strings.Contains(clean, "..") {
		return nil, fmt.Errorf("invalid path: contains directory traversal")
	}
	return os.ReadFile(clean)
}
