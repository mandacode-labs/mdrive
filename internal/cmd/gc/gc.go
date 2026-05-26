// Package gc implements the gc command
package gc

import (
	"context"
	"fmt"
	"time"

	"github.com/mandacode-labs/retrowin-go/ent"
	gcapp "github.com/mandacode-labs/retrowin-go/internal/application/gc"

	"github.com/mandacode-labs/retrowin-go/internal/config"
	"github.com/mandacode-labs/retrowin-go/internal/core/object"
	objectrepo "github.com/mandacode-labs/retrowin-go/internal/core/object/repository"
	s3storage "github.com/mandacode-labs/retrowin-go/internal/core/object/s3"
)

// runGC bootstraps dependencies and runs garbage collection.
func runGC(cfg *config.Config, pendingExpiry time.Duration) error {
	ctx := context.Background()

	// Create DB client (same pattern as migrate command)
	entClient, err := ent.Open(cfg.Database.Driver, cfg.DSN())
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = entClient.Close() }()

	// Create S3 storage
	objStorage, err := s3storage.New(&cfg.Storage)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	// Build object service
	objectSvc := object.NewService(objectrepo.NewRepository(entClient), objStorage)

	// Run GC
	collector := gcapp.NewGarbageCollector(objectSvc, objStorage, pendingExpiry)

	fmt.Println("Running garbage collection...")
	result, err := collector.Run(ctx)
	if err != nil {
		return fmt.Errorf("garbage collection failed: %w", err)
	}

	fmt.Printf("GC complete: %d pending cleaned, %d orphans cleaned\n",
		result.PendingCleaned, result.OrphansCleaned)
	return nil
}
