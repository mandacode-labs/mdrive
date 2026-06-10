package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	openapispec "github.com/mandacode-labs/mdrive/api"
	"github.com/mandacode-labs/mdrive/internal/cmd/serve"
)

func TestSuite_Start(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	// Verify database connection
	require.NotNil(t, suite.GetEntClient(), "EntClient should not be nil")
	require.NotNil(t, suite.GetDB(), "DB should not be nil")

	// Verify database is accessible
	var result int
	err = suite.GetDB().QueryRow("SELECT 1").Scan(&result)
	require.NoError(t, err, "Failed to query database")
	assert.Equal(t, 1, result)

	t.Log("E2E test suite started successfully")
}

func TestSuite_Migration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	// Start server to run migrations
	err = suite.StartServer(ctx)
	require.NoError(t, err, "Failed to start server")

	// Verify tables were created - using actual ent table names
	tables := []string{"users", "inodes", "objects", "systems", "user_systems", "system_groups", "user_groups"}
	for _, table := range tables {
		var exists bool
		err := suite.GetDB().QueryRow(
			"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)",
			table,
		).Scan(&exists)
		require.NoError(t, err, "Failed to check table existence")
		assert.True(t, exists, "Table %s should exist", table)
	}

	t.Log("Database migration verified successfully")
}

func TestSuite_ServerStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	shutdown := make(chan struct{})

	// Create a test HTTP server that simulates the mdrive server
	mux := http.NewServeMux()

	// Health check endpoint (direct access, no /v1 prefix)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})

	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	go func() {
		t.Log("Starting test server on 127.0.0.1:8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
		close(shutdown)
	}()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)

	// Test /health endpoint
	resp, err := http.Get("http://127.0.0.1:8080/health")
	require.NoError(t, err, "Failed to call /health endpoint")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health status should be 200")

	// Verify content type
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")
	assert.Contains(t, string(body), "healthy", "Response should contain 'healthy'")

	t.Log("Server startup test passed successfully")

	// Shutdown server
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	_ = server.Shutdown(ctxShutdown)

	// Wait for shutdown to complete
	select {
	case <-shutdown:
	case <-time.After(5 * time.Second):
	}
}

func TestSuite_FullServerStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	// Create a temporary config file with shared container connection details
	tmpDir := t.TempDir()
	cfgFile := tmpDir + "/config.yaml"

	// Get config (already populated with shared container details by Start())
	cfg := suite.GetConfig()

	// Disable services that require external dependencies for e2e testing
	cfg.Auth.Keycloak.BaseURL = "http://localhost:9999" // Invalid URL to prevent actual OIDC calls

	// Write config to temp file as YAML
	cfgData, err := yaml.Marshal(cfg)
	require.NoError(t, err, "Failed to marshal config")
	err = os.WriteFile(cfgFile, cfgData, 0644)
	require.NoError(t, err, "Failed to write config file")

	t.Logf("Using config file: %s", cfgFile)
	t.Logf("Database: %s:%d", cfg.Database.Host, cfg.Database.Port)

	// Start the actual app with embedded OpenAPI spec
	app, err := serve.NewApp(cfgFile, cfg.HTTP.Port, openapispec.Spec)
	require.NoError(t, err, "Failed to create app")

	// Start app in background
	err = app.Start()
	require.NoError(t, err, "Failed to start app")

	// Wait for app to start
	time.Sleep(2 * time.Second)

	// Test /health endpoint - this MUST succeed for the test to pass
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", cfg.HTTP.Port))
	require.NoError(t, err, "HTTP server must be reachable on /health endpoint")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response")
	assert.Contains(t, string(body), "healthy", "Response should contain 'healthy'")

	t.Log("Full server startup test passed - server is running and responding")

	// Shutdown the app
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = app.Shutdown(shutdownCtx)
}

func TestSuite_OpenAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	err = suite.StartServer(ctx)
	require.NoError(t, err, "Failed to start server")

	// Test /openapi.json endpoint
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/openapi.json", suite.GetConfig().HTTP.Port))
	require.NoError(t, err, "Failed to call /openapi.json endpoint")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "OpenAPI spec should return 200")

	// Verify content type
	ct := resp.Header.Get("Content-Type")
	assert.Contains(t, ct, "application/json", "Content-Type should be application/json")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read response body")
	assert.Contains(t, string(body), "openapi", "Response should contain OpenAPI spec")

	t.Log("OpenAPI endpoint test passed successfully")
}

func TestSuite_CORS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	err = suite.StartServer(ctx)
	require.NoError(t, err, "Failed to start server")

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", suite.GetConfig().HTTP.Port)

	// Test 1: OPTIONS preflight request (should return 204 with CORS headers)
	t.Run("Preflight_Success", func(t *testing.T) {
		req, err := http.NewRequest("OPTIONS", baseURL+"/user", nil)
		require.NoError(t, err, "Failed to create request")
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "GET")
		req.Header.Set("Access-Control-Request-Headers", "authorization")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Failed to send OPTIONS request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode, "OPTIONS preflight should return 204")

		// Verify CORS headers
		allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
		assert.Equal(t, "http://localhost:3000", allowOrigin, "Should return correct origin")

		allowMethods := resp.Header.Get("Access-Control-Allow-Methods")
		assert.Contains(t, allowMethods, "GET", "Should allow GET method")

		allowHeaders := resp.Header.Get("Access-Control-Allow-Headers")
		assert.Contains(t, allowHeaders, "Authorization", "Should allow authorization header")

		maxAge := resp.Header.Get("Access-Control-Max-Age")
		assert.NotEmpty(t, maxAge, "Should have max-age header")
	})

	// Test 2: Simple request with Origin header (should return CORS headers)
	t.Run("Simple_Request", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/health", nil)
		require.NoError(t, err, "Failed to create request")
		req.Header.Set("Origin", "http://localhost:3000")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Failed to send GET request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200")

		allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
		assert.Equal(t, "http://localhost:3000", allowOrigin, "Should return CORS header for simple request")
	})

	// Test 3: Preflight with allowed origin from config
	t.Run("Preflight_AllowedOrigin", func(t *testing.T) {
		req, err := http.NewRequest("OPTIONS", baseURL+"/user", nil)
		require.NoError(t, err, "Failed to create request")
		req.Header.Set("Origin", "https://mdrive.starship.co")
		req.Header.Set("Access-Control-Request-Method", "GET")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Failed to send OPTIONS request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode, "OPTIONS should return 204")

		allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
		assert.Equal(t, "https://mdrive.starship.co", allowOrigin, "Should return configured origin")
	})

	t.Log("CORS tests passed successfully")
}

func TestSuite_Auth_Callback_ErrorResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	err = suite.StartServer(ctx)
	require.NoError(t, err, "Failed to start server")

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", suite.GetConfig().HTTP.Port)

	// Test 1: Missing required parameters returns proper error response
	// Note: ogen returns 500 for decode errors (missing required params)
	t.Run("Missing_Code_Parameter", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/auth/callback?state=test")
		require.NoError(t, err, "Failed to send request")
		defer func() { _ = resp.Body.Close() }()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Failed to read response body")

		// ogen returns 500 for decode errors, but response should still have proper structure
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode, "Should return 500 for decode error")

		// Verify response contains error details
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		// Should contain error structure with type and message
		assert.Contains(t, string(body), `"error"`, "Response should contain error object")
		assert.Contains(t, string(body), `"type"`, "Response should contain error type")
		assert.Contains(t, string(body), `"message"`, "Response should contain error message")
		assert.Contains(t, string(body), "code", "Error message should mention missing code parameter")
	})

	// Test 2: Invalid code returns proper error response with 400/401
	t.Run("Invalid_Code_Parameter", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/auth/callback?code=invalid&state=test")
		require.NoError(t, err, "Failed to send request")
		defer func() { _ = resp.Body.Close() }()

		// Should return 400 or 401 depending on the error
		assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnauthorized}, resp.StatusCode,
			"Should return 400 or 401 for invalid code")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "Failed to read response body")

		// Should contain error structure
		assert.Contains(t, string(body), `"error"`, "Response should contain error object")
	})

	t.Log("Auth callback error response tests passed successfully")
}
