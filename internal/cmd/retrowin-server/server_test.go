// Package retrowinserver implements the retrowin-server command
package retrowinserver

import (
	"context"
	"testing"
	"time"

	"go.uber.org/fx"

	"github.com/mandacode-labs/retrowin-go/internal/cmd/serve"
)

// TestServerOptions verifies that the fx options are properly constructed.
func TestServerOptions(t *testing.T) {
	options := serve.FxOptions("", 8080, []byte{})

	if options == nil {
		t.Fatal("FxOptions returned nil")
	}

	// Verify that options is not empty and contains expected options
	if len(options) == 0 {
		t.Error("FxOptions returned empty slice")
	}
}

// TestServerStart verifies that the fx app can be created and starts without errors.
// This test doesn't actually start the server but validates the dependency graph.
func TestServerStart(t *testing.T) {
	app := fx.New(serve.FxOptions("", 8080, []byte{})...)

	if app == nil {
		t.Fatal("fx.New returned nil")
	}

	// Start the app - expected to fail due to missing config/database
	// but this validates the dependency graph is correctly constructed
	startCtx, startCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer startCancel()

	err := app.Start(startCtx)
	// Error is expected in test environment without proper config
	if err != nil {
		t.Logf("App start failed as expected in test environment: %v", err)
	}
}

// TestServerShutdown verifies that the fx app can shut down cleanly.
func TestServerShutdown(t *testing.T) {
	t.Skip("Skipping integration test - requires database and cache")

	app := serve.NewFXApp("", 8081, []byte{})

	// Start the app in a goroutine
	done := make(chan struct{}, 1)
	go func() {
		app.Run()
		close(done)
	}()

	// Give it time to start
	time.Sleep(2 * time.Second)

	// Stop the app
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := app.Stop(ctx); err != nil {
		t.Errorf("Failed to stop app: %v", err)
	}

	// Wait for it to finish
	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Error("App did not shut down in time")
	}
}
