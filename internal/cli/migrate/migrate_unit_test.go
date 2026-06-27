package migrate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBConfigURL(t *testing.T) {
	tests := []struct {
		name    string
		cfg     dbConfig
		want    string
		wantErr error
	}{
		{
			name: "host only",
			cfg:  dbConfig{Host: "localhost", Name: "mdrive"},
			want: "postgres://localhost/mdrive",
		},
		{
			name: "host with port and sslmode",
			cfg: dbConfig{
				Host:    "db.example.com",
				Port:    5432,
				Name:    "mdrive",
				User:    "mdrive",
				SSLMode: "require",
			},
			// net/url emits an empty-password colon when User is set with
			// empty Password; libpq accepts it as "no password supplied".
			want: "postgres://mdrive:@db.example.com:5432/mdrive?sslmode=require",
		},
		{
			name: "user and password",
			cfg: dbConfig{
				Host:     "localhost",
				Name:     "mdrive",
				User:     "mdrive-app",
				Password: "secret",
			},
			want: "postgres://mdrive-app:secret@localhost/mdrive",
		},
		{
			name: "password with URL-unsafe characters gets escaped",
			cfg: dbConfig{
				Host:     "localhost",
				Name:     "mdrive",
				User:     "mdrive",
				Password: "p@ss:w/rd",
			},
			want: "postgres://mdrive:p%40ss%3Aw%2Frd@localhost/mdrive",
		},
		{
			name: "empty password renders as empty-password colon",
			cfg: dbConfig{
				Host: "localhost",
				Name: "mdrive",
				User: "mdrive",
			},
			want: "postgres://mdrive:@localhost/mdrive",
		},
		{
			name:    "missing host returns error",
			cfg:     dbConfig{Name: "mdrive"},
			wantErr: errHostRequired,
		},
		{
			name:    "missing name returns error",
			cfg:     dbConfig{Host: "localhost"},
			wantErr: errNameRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.url()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveDBConfig(t *testing.T) {
	// Snapshot env state so the test is hermetic.
	keys := []string{envHost, envPort, envName, envUser, envPassword, envSSLMode}
	original := map[string]string{}
	for _, k := range keys {
		original[k] = os.Getenv(k)
		_ = os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for _, k := range keys {
			if original[k] != "" {
				_ = os.Setenv(k, original[k])
			} else {
				_ = os.Unsetenv(k)
			}
		}
	})

	t.Run("flags only", func(t *testing.T) {
		c := resolveDBConfig("h", "n", "u", "p", "ssl", 5432)
		assert.Equal(t, dbConfig{
			Host: "h", Port: 5432, Name: "n",
			User: "u", Password: "p", SSLMode: "ssl",
		}, c)
	})

	t.Run("env only", func(t *testing.T) {
		_ = os.Setenv(envHost, "envhost")
		_ = os.Setenv(envPort, "5433")
		_ = os.Setenv(envName, "envname")
		_ = os.Setenv(envUser, "envuser")
		_ = os.Setenv(envPassword, "envpass")
		_ = os.Setenv(envSSLMode, "disable")

		c := resolveDBConfig("", "", "", "", "", 0)
		assert.Equal(t, dbConfig{
			Host: "envhost", Port: 5433, Name: "envname",
			User: "envuser", Password: "envpass", SSLMode: "disable",
		}, c)
	})

	t.Run("flags override env", func(t *testing.T) {
		_ = os.Setenv(envHost, "envhost")
		_ = os.Setenv(envPort, "5433")
		_ = os.Setenv(envName, "envname")
		_ = os.Setenv(envUser, "envuser")
		_ = os.Setenv(envPassword, "envpass")
		_ = os.Setenv(envSSLMode, "disable")

		c := resolveDBConfig("flaghost", "flagname", "flaguser", "flagpass", "require", 1111)
		assert.Equal(t, dbConfig{
			Host: "flaghost", Port: 1111, Name: "flagname",
			User: "flaguser", Password: "flagpass", SSLMode: "require",
		}, c)
	})

	t.Run("invalid port env falls back to zero", func(t *testing.T) {
		_ = os.Setenv(envPort, "not-a-number")
		c := resolveDBConfig("", "", "", "", "", 0)
		assert.Equal(t, 0, c.Port)
	})
}

func TestFirstNonEmpty(t *testing.T) {
	assert.Equal(t, "", firstNonEmpty())
	assert.Equal(t, "a", firstNonEmpty("", "a", "b"))
	assert.Equal(t, "a", firstNonEmpty("a", "b"))
	assert.Equal(t, "b", firstNonEmpty("", "", "b"))
}
