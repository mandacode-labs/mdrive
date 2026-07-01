// Package migrate implements the `mdrive migrate` CLI subcommand.
package migrate

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"strconv"

	"ariga.io/atlas-go-sdk/atlasexec"
	"github.com/spf13/cobra"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

//go:embed migrations/*.sql migrations/atlas.sum
var defaultMigrations embed.FS

const defaultAtlasBin = "atlas"

// Environment variable names for individual database connection
// parameters. These follow the project convention (viper
// SetEnvKeyReplacer(".", "_")) so the chart can inject DATABASE_*
// directly without building a URL at render time.
const (
	envHost     = "DATABASE_HOST"
	envPort     = "DATABASE_PORT"
	envName     = "DATABASE_NAME"
	envUser     = "DATABASE_USER"
	envPassword = "DATABASE_PASSWORD"
	envSSLMode  = "DATABASE_SSLMODE"
)

// dbConfig holds the resolved connection parameters used to
// build the atlas-compatible URL. The CLI accepts each field as
// either a flag or a DATABASE_* environment variable. Flags
// take precedence over the environment.
type dbConfig struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	SSLMode  string
}

// url builds a libpq-compatible URL using net/url so the password
// is properly URL-encoded. This is the single point that converts
// our connection struct to what atlas expects.
func (c dbConfig) url() (string, error) {
	if c.Host == "" {
		return "", errHostRequired
	}
	if c.Name == "" {
		return "", errNameRequired
	}
	u := url.URL{
		Scheme: "postgres",
		Path:   "/" + c.Name,
	}
	if c.User != "" || c.Password != "" {
		u.User = url.UserPassword(c.User, c.Password)
	}
	host := c.Host
	if c.Port > 0 {
		host = c.Host + ":" + strconv.Itoa(c.Port)
	}
	u.Host = host
	if c.SSLMode != "" {
		u.RawQuery = "sslmode=" + url.QueryEscape(c.SSLMode)
	}
	return u.String(), nil
}

// resolveDBConfig merges flag overrides over environment variables.
// Flags take precedence over the environment.
func resolveDBConfig(flagHost, flagName, flagUser, flagPassword, flagSSLMode string, flagPort int) dbConfig {
	c := dbConfig{
		Host:     firstNonEmpty(flagHost, os.Getenv(envHost)),
		Name:     firstNonEmpty(flagName, os.Getenv(envName)),
		User:     firstNonEmpty(flagUser, os.Getenv(envUser)),
		Password: firstNonEmpty(flagPassword, os.Getenv(envPassword)),
		SSLMode:  firstNonEmpty(flagSSLMode, os.Getenv(envSSLMode)),
	}
	if flagPort > 0 {
		c.Port = flagPort
	} else if p := os.Getenv(envPort); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			c.Port = n
		}
	}
	return c
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

var (
	errHostRequired = errors.New("migrate: DATABASE_HOST (or --database-host) is required")
	errNameRequired = errors.New("migrate: DATABASE_NAME (or --database-name) is required")
)

func NewCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database migrations",
	}
	root.AddCommand(newApplyCmd())
	return root
}

// flagSet holds the bound flag values so we can pass them through
// to resolveDBConfig without leaking cobra types into the testable
// core.
type flagSet struct {
	host     string
	port     int
	name     string
	user     string
	password string
	sslmode  string
}

func bindDBFlags(cmd *cobra.Command, f *flagSet) {
	cmd.Flags().StringVar(&f.host, "database-host", "",
		"PostgreSQL host (env: DATABASE_HOST)")
	cmd.Flags().IntVar(&f.port, "database-port", 0,
		"PostgreSQL port (env: DATABASE_PORT)")
	cmd.Flags().StringVar(&f.name, "database-name", "",
		"PostgreSQL database name (env: DATABASE_NAME)")
	cmd.Flags().StringVar(&f.user, "database-user", "",
		"PostgreSQL user (env: DATABASE_USER)")
	cmd.Flags().StringVar(&f.password, "database-password", "",
		"PostgreSQL password (env: DATABASE_PASSWORD)")
	cmd.Flags().StringVar(&f.sslmode, "database-sslmode", "",
		"PostgreSQL sslmode (env: DATABASE_SSLMODE)")
}

func newApplyCmd() *cobra.Command {
	flags := &flagSet{}
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply pending Atlas versioned migrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := resolveDBConfig(flags.host, flags.name, flags.user, flags.password, flags.sslmode, flags.port)
			databaseURL, err := cfg.url()
			if err != nil {
				return err
			}
			return apply(cmd.Context(), databaseURL)
		},
	}
	bindDBFlags(cmd, flags)
	return cmd
}

func apply(ctx context.Context, databaseURL string) error {
	migrations, err := fs.Sub(defaultMigrations, "migrations")
	if err != nil {
		return errorx.Wrap(err, "migrate: fs sub")
	}
	return applyWith(ctx, databaseURL, migrations, defaultAtlasBin)
}

func applyWith(ctx context.Context, databaseURL string, migrations fs.FS, atlasBin string) error {
	workDir, err := atlasexec.NewWorkingDir(atlasexec.WithMigrations(migrations))
	if err != nil {
		return errorx.Wrap(err, "migrate: atlas working dir")
	}
	defer func() { _ = workDir.Close() }()

	client, err := atlasexec.NewClient(workDir.Path(), atlasBin)
	if err != nil {
		return errorx.Wrap(err, "migrate: atlas client (bin=%s)", atlasBin)
	}

	if _, err := client.MigrateApply(ctx, &atlasexec.MigrateApplyParams{
		URL: databaseURL,
	}); err != nil {
		return errorx.Wrap(err, "migrate: atlas apply")
	}
	return nil
}
