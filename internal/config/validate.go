package config

import (
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		return errors.BadRequest("invalid HTTP port")
	}

	if c.Database.Host == "" {
		return errors.BadRequest("database host is required")
	}

	if c.Database.Name == "" {
		return errors.BadRequest("database name is required")
	}

	if c.Storage.Bucket == "" {
		return errors.BadRequest("storage bucket is required")
	}

	return nil
}
