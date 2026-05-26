package e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFs_Stat tests inode stat operations
func TestFs_Stat(t *testing.T) {
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

	// Setup user and system via API (for proper filesystem initialization)
	_, systemData, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := systemData["system"].(map[string]any)["id"].(string)

	t.Run("returns root directory info", func(t *testing.T) {
		resp, err := suite.Get("/fs/" + systemID + "/root")
		require.NoError(t, err, "Failed to stat root")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusOK, resp.StatusCode,
			"Expected 200 OK, got %d", resp.StatusCode)

		var result map[string]any
		err = suite.ReadJSON(resp, &result)
		require.NoError(t, err, "Failed to read response JSON")

		// Check inode field
		inode, ok := result["inode"].(map[string]any)
		require.True(t, ok, "Response should contain inode object")

		// Verify root directory attributes
		assert.NotEmpty(t, inode["id"], "Root should have an ID")
		assert.Equal(t, systemID, inode["systemId"], "System ID should match")

		// Mode should be a directory (040000 = S_IFDIR)
		mode, ok := inode["mode"].(float64)
		require.True(t, ok, "Mode should be a number")
		assert.NotZero(t, int(mode)&040000, "Should be a directory")
	})

	t.Run("returns inode by path", func(t *testing.T) {
		// Use query parameter for path
		resp, err := suite.Get("/fs/" + systemID + "/stat?path=" + url.QueryEscape("/home"))
		require.NoError(t, err, "Failed to stat /home")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 OK")

		var result map[string]any
		err = suite.ReadJSON(resp, &result)
		require.NoError(t, err, "Failed to read response JSON")

		inode, ok := result["inode"].(map[string]any)
		require.True(t, ok, "Response should contain inode object")
		assert.NotEmpty(t, inode["id"], "Inode should have an ID")
	})

	t.Run("returns 404 for non-existent path", func(t *testing.T) {
		resp, err := suite.Get("/fs/" + systemID + "/stat?path=" + url.QueryEscape("/nonexistent/path"))
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"Expected 404 Not Found, got %d", resp.StatusCode)
	})
}

// TestFs_Ls tests directory listing
func TestFs_Ls(t *testing.T) {
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

	// Setup user and system via API
	_, systemData, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := systemData["system"].(map[string]any)["id"].(string)

	t.Run("lists root directory", func(t *testing.T) {
		resp, err := suite.Get("/syscall/" + systemID + "/ls?path=/")
		require.NoError(t, err, "Failed to read root directory")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusOK, resp.StatusCode,
			"Expected 200 OK, got %d", resp.StatusCode)

		var result map[string]any
		err = suite.ReadJSON(resp, &result)
		require.NoError(t, err, "Failed to read response JSON")

		// Check entries array
		entries, ok := result["entries"].([]any)
		require.True(t, ok, "Response should contain entries array")

		// Root should have at least home directory
		assert.GreaterOrEqual(t, len(entries), 1,
			"Root should have at least home directory")
	})

	t.Run("lists directory by path", func(t *testing.T) {
		resp, err := suite.Get("/syscall/" + systemID + "/ls?path=" + url.QueryEscape("/home"))
		require.NoError(t, err, "Failed to read /home")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusOK, resp.StatusCode,
			"Expected 200 OK, got %d", resp.StatusCode)

		var result map[string]any
		err = suite.ReadJSON(resp, &result)
		require.NoError(t, err, "Failed to read response JSON")

		// Should have entries
		entries, ok := result["entries"].([]any)
		require.True(t, ok, "Response should contain entries array")
		_ = entries
	})

	t.Run("returns 404 for non-existent directory", func(t *testing.T) {
		resp, err := suite.Get("/syscall/" + systemID + "/ls?path=" + url.QueryEscape("/nonexistent/dir"))
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"Expected 404 Not Found, got %d", resp.StatusCode)
	})
}

// TestFs_Mkdir tests directory creation
func TestFs_Mkdir(t *testing.T) {
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

	// Setup user and system via API
	_, systemData, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := systemData["system"].(map[string]any)["id"].(string)

	t.Run("creates directory with default permissions", func(t *testing.T) {
		req := map[string]any{
			"path": "/home/newdir",
		}

		resp, err := suite.Post("/syscall/"+systemID+"/mkdir", req)
		require.NoError(t, err, "Failed to create directory")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusCreated, resp.StatusCode,
			"Expected 201 Created, got %d: %s", resp.StatusCode, suite.ReadBody(resp))

		// Verify directory was created
		statResp, err := suite.Get("/fs/" + systemID + "/stat?path=" + url.QueryEscape("/home/newdir"))
		require.NoError(t, err)
		defer func() { _ = statResp.Body.Close() }()
		require.Equal(t, http.StatusOK, statResp.StatusCode)
	})

	t.Run("creates directory with custom permissions", func(t *testing.T) {
		req := map[string]any{
			"path": "/home/privatedir",
			"mode": 0700,
		}

		resp, err := suite.Post("/syscall/"+systemID+"/mkdir", req)
		require.NoError(t, err, "Failed to create directory")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusCreated, resp.StatusCode,
			"Expected 201 Created, got %d", resp.StatusCode)

		// Verify permissions
		statResp, err := suite.Get("/fs/" + systemID + "/stat?path=" + url.QueryEscape("/home/privatedir"))
		require.NoError(t, err)
		var result map[string]any
		_ = suite.ReadJSON(statResp, &result)
		_ = statResp.Body.Close()

		inode, ok := result["inode"].(map[string]any)
		require.True(t, ok, "Response should contain inode")
		mode := int(inode["mode"].(float64))
		assert.Equal(t, 0700, mode&0777, "Permissions should be 0700")
	})

	t.Run("rejects duplicate directory", func(t *testing.T) {
		req := map[string]any{
			"path": "/home/dupdir",
		}

		// First create should succeed
		resp1, err := suite.Post("/syscall/"+systemID+"/mkdir", req)
		require.NoError(t, err)
		_ = resp1.Body.Close()
		require.Equal(t, http.StatusCreated, resp1.StatusCode)

		// Second create should fail
		resp2, err := suite.Post("/syscall/"+systemID+"/mkdir", req)
		require.NoError(t, err)
		defer func() { _ = resp2.Body.Close() }()

		assert.Equal(t, http.StatusConflict, resp2.StatusCode,
			"Expected 409 Conflict for duplicate, got %d", resp2.StatusCode)
	})

	t.Run("rejects dot and dotdot names", func(t *testing.T) {
		for _, name := range []string{".", ".."} {
			req := map[string]any{
				"path": "/home/" + name,
			}
			resp, err := suite.Post("/syscall/"+systemID+"/mkdir", req)
			require.NoError(t, err)
			_ = resp.Body.Close()
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
				"Expected 400 for name %q, got %d", name, resp.StatusCode)
		}
	})

	t.Run("creates directory without directory bit in mode", func(t *testing.T) {
		// Sending mode without S_IFDIR should still create a directory
		req := map[string]any{
			"path": "/home/nodirbit",
			"mode": 0755, // no S_IFDIR bit
		}

		resp, err := suite.Post("/syscall/"+systemID+"/mkdir", req)
		require.NoError(t, err, "Failed to create directory")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusCreated, resp.StatusCode,
			"Expected 201 Created, got %d: %s", resp.StatusCode, suite.ReadBody(resp))

		// Verify it is actually a directory by listing it
		lsResp, err := suite.Get("/syscall/" + systemID + "/ls?path=" + url.QueryEscape("/home/nodirbit"))
		require.NoError(t, err)
		defer func() { _ = lsResp.Body.Close() }()
		assert.Equal(t, http.StatusOK, lsResp.StatusCode,
			"Expected 200 OK for ls, got %d", lsResp.StatusCode)
	})

	t.Run("creates directory with plus in name", func(t *testing.T) {
		req := map[string]any{
			"path": "/home/New+Folder",
		}

		resp, err := suite.Post("/syscall/"+systemID+"/mkdir", req)
		require.NoError(t, err, "Failed to create directory")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusCreated, resp.StatusCode,
			"Expected 201 Created, got %d: %s", resp.StatusCode, suite.ReadBody(resp))

		// Verify directory exists and is listable
		lsResp, err := suite.Get("/syscall/" + systemID + "/ls?path=" + url.QueryEscape("/home/New+Folder"))
		require.NoError(t, err)
		defer func() { _ = lsResp.Body.Close() }()
		assert.Equal(t, http.StatusOK, lsResp.StatusCode,
			"Expected 200 OK for ls, got %d", lsResp.StatusCode)
	})
}

// TestFs_Delete tests inode deletion
func TestFs_Delete(t *testing.T) {
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

	// Setup user and system via API
	_, systemData, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := systemData["system"].(map[string]any)["id"].(string)

	t.Run("deletes empty directory", func(t *testing.T) {
		// Create directory first
		mkdirReq := map[string]any{
			"path": "/home/deletedir",
		}
		mkdirResp, err := suite.Post("/syscall/"+systemID+"/mkdir", mkdirReq)
		require.NoError(t, err)
		_ = mkdirResp.Body.Close()
		require.Equal(t, http.StatusCreated, mkdirResp.StatusCode)

		// Delete the directory
		deleteResp, err := suite.Delete("/syscall/" + systemID + "/unlink?path=" + url.QueryEscape("/home/deletedir"))
		require.NoError(t, err, "Failed to delete directory")
		defer func() { _ = deleteResp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode,
			"Expected 204 No Content, got %d", deleteResp.StatusCode)

		// Verify directory is deleted
		statResp, err := suite.Get("/fs/" + systemID + "/stat?path=" + url.QueryEscape("/home/deletedir"))
		require.NoError(t, err)
		_ = statResp.Body.Close()
		assert.Equal(t, http.StatusNotFound, statResp.StatusCode,
			"Directory should not exist after deletion")
	})

	t.Run("rejects non-empty directory", func(t *testing.T) {
		// Create directory with a subdirectory inside
		mkdirReq := map[string]any{
			"path": "/home/nonemptydir",
		}
		mkdirResp, err := suite.Post("/syscall/"+systemID+"/mkdir", mkdirReq)
		require.NoError(t, err)
		_ = mkdirResp.Body.Close()

		// Create subdirectory
		subReq := map[string]any{
			"path": "/home/nonemptydir/subdir",
		}
		subResp, err := suite.Post("/syscall/"+systemID+"/mkdir", subReq)
		require.NoError(t, err)
		_ = subResp.Body.Close()

		// Try to delete non-empty directory
		deleteResp, err := suite.Delete("/syscall/" + systemID + "/unlink?path=" + url.QueryEscape("/home/nonemptydir"))
		require.NoError(t, err, "Failed to make delete request")
		defer func() { _ = deleteResp.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, deleteResp.StatusCode,
			"Expected 400 Bad Request for non-empty directory, got %d", deleteResp.StatusCode)
	})

	t.Run("returns 404 for non-existent path", func(t *testing.T) {
		deleteResp, err := suite.Delete("/syscall/" + systemID + "/unlink?path=" + url.QueryEscape("/home/nonexistent"))
		require.NoError(t, err, "Failed to make delete request")
		defer func() { _ = deleteResp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, deleteResp.StatusCode,
			"Expected 404 Not Found, got %d", deleteResp.StatusCode)
	})
}

// TestFs_Ln tests symbolic link creation
func TestFs_Ln(t *testing.T) {
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

	// Setup user and system via API
	_, systemData, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := systemData["system"].(map[string]any)["id"].(string)

	// Create a target directory first (symlink target needs to exist for useful test)
	mkdirReq := map[string]any{
		"path": "/home/targetdir",
	}
	mkdirResp, err := suite.Post("/syscall/"+systemID+"/mkdir", mkdirReq)
	require.NoError(t, err)
	_ = mkdirResp.Body.Close()
	require.Equal(t, http.StatusCreated, mkdirResp.StatusCode)

	t.Run("creates symbolic link", func(t *testing.T) {
		req := map[string]any{
			"target":   "/home/targetdir",
			"linkPath": "/home/linkdir",
		}

		resp, err := suite.Post("/syscall/"+systemID+"/ln", req)
		require.NoError(t, err, "Failed to create symlink")

		// Read body first (before any assertion that might consume it)
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.NoError(t, err, "Failed to read body")

		t.Logf("Symlink status=%d body=%s", resp.StatusCode, string(body))

		require.Equal(t, http.StatusCreated, resp.StatusCode,
			"Expected 201 Created, got %d: %s", resp.StatusCode, string(body))

		// Verify symlink was created from the POST response
		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("Failed to parse JSON: %v, body: %s", err, string(body))
		}

		inode, ok := result["inode"].(map[string]any)
		keys := make([]string, 0, len(result))
		for k := range result {
			keys = append(keys, k)
		}
		require.True(t, ok, "Response should contain inode, got keys: %v", keys)

		// Mode should indicate symlink (0120000 = S_IFLNK)
		mode := int(inode["mode"].(float64))
		assert.NotZero(t, mode&0120000, "Should be a symlink (mode=%o)", mode) // S_IFLNK

		// Note: stat follows symlinks, so /home/linkdir would resolve to the target directory
	})

	t.Run("can create dangling symlink", func(t *testing.T) {
		req := map[string]any{
			"target":   "/home/nonexistent.txt",
			"linkPath": "/home/dangling.txt",
		}

		resp, err := suite.Post("/syscall/"+systemID+"/ln", req)
		require.NoError(t, err, "Failed to create dangling symlink")
		defer func() { _ = resp.Body.Close() }()

		// Dangling symlinks should still be created successfully
		require.Equal(t, http.StatusCreated, resp.StatusCode,
			"Symlink to non-existent target should still be created")
	})
}

// TestFs_Chmod tests permission changes
func TestFs_Chmod(t *testing.T) {
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

	// Setup user and system via API
	_, systemData, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := systemData["system"].(map[string]any)["id"].(string)

	// Create a directory first
	mkdirReq := map[string]any{
		"path": "/home/chmoddir",
		"mode": 0755,
	}
	mkdirResp, err := suite.Post("/syscall/"+systemID+"/mkdir", mkdirReq)
	require.NoError(t, err)
	_ = mkdirResp.Body.Close()
	require.Equal(t, http.StatusCreated, mkdirResp.StatusCode)

	t.Run("changes directory permissions", func(t *testing.T) {
		req := map[string]any{
			"path": "/home/chmoddir",
			"mode": 0700,
		}

		resp, err := suite.Patch("/fs/"+systemID+"/chmod", req)
		require.NoError(t, err, "Failed to chmod")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusOK, resp.StatusCode,
			"Expected 200 OK, got %d", resp.StatusCode)

		// Verify permissions changed
		statResp, err := suite.Get("/fs/" + systemID + "/stat?path=" + url.QueryEscape("/home/chmoddir"))
		require.NoError(t, err)
		var result map[string]any
		_ = suite.ReadJSON(statResp, &result)
		_ = statResp.Body.Close()

		inode, ok := result["inode"].(map[string]any)
		require.True(t, ok, "Response should contain inode")
		mode := int(inode["mode"].(float64))
		assert.Equal(t, 0700, mode&0777, "Permissions should be 0700")
	})

	t.Run("returns 404 for non-existent path", func(t *testing.T) {
		req := map[string]any{
			"path": "/home/nonexistent",
			"mode": 0755,
		}

		resp, err := suite.Patch("/fs/"+systemID+"/chmod", req)
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"Expected 404 Not Found, got %d", resp.StatusCode)
	})
}

// TestFs_Permission tests permission enforcement
func TestFs_Permission(t *testing.T) {
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

	// Setup user and system via API
	_, systemData, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := systemData["system"].(map[string]any)["id"].(string)

	// Create a directory with restricted permissions
	mkdirReq := map[string]any{
		"path": "/home/owneronly",
		"mode": 0700,
	}
	mkdirResp, err := suite.Post("/syscall/"+systemID+"/mkdir", mkdirReq)
	require.NoError(t, err)
	_ = mkdirResp.Body.Close()
	require.Equal(t, http.StatusCreated, mkdirResp.StatusCode)

	t.Run("owner can access owner-only directory", func(t *testing.T) {
		// Owner should be able to stat the directory
		statResp, err := suite.Get("/fs/" + systemID + "/stat?path=" + url.QueryEscape("/home/owneronly"))
		require.NoError(t, err)
		defer func() { _ = statResp.Body.Close() }()
		require.Equal(t, http.StatusOK, statResp.StatusCode)
	})

	// TODO: Add tests for:
	// - group can read group-readable file
	// - others cannot read owner-only file
	// - root can access any file
	// These require additional user setup
}

// TestFs_Rename tests file/directory renaming
func TestFs_Rename(t *testing.T) {
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

	// Setup user and system via API
	_, systemData, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := systemData["system"].(map[string]any)["id"].(string)

	t.Run("renames directory within same parent", func(t *testing.T) {
		// Create directory first
		mkdirReq := map[string]any{
			"path": "/home/olddir",
		}
		mkdirResp, err := suite.Post("/syscall/"+systemID+"/mkdir", mkdirReq)
		require.NoError(t, err)
		_ = mkdirResp.Body.Close()
		require.Equal(t, http.StatusCreated, mkdirResp.StatusCode)

		// Rename the directory
		renameReq := map[string]any{
			"path":    "/home/olddir",
			"newName": "newdir",
		}
		renameResp, err := suite.Post("/syscall/"+systemID+"/rename", renameReq)
		require.NoError(t, err, "Failed to rename directory")
		defer func() { _ = renameResp.Body.Close() }()

		require.Equal(t, http.StatusOK, renameResp.StatusCode,
			"Expected 200 OK, got %d", renameResp.StatusCode)

		// Verify old path doesn't exist
		statOldResp, err := suite.Get("/fs/" + systemID + "/stat?path=" + url.QueryEscape("/home/olddir"))
		require.NoError(t, err)
		_ = statOldResp.Body.Close()
		assert.Equal(t, http.StatusNotFound, statOldResp.StatusCode,
			"Old path should not exist after rename")

		// Verify new path exists
		statNewResp, err := suite.Get("/fs/" + systemID + "/stat?path=" + url.QueryEscape("/home/newdir"))
		require.NoError(t, err)
		_ = statNewResp.Body.Close()
		assert.Equal(t, http.StatusOK, statNewResp.StatusCode,
			"New path should exist after rename")
	})

	t.Run("returns 404 for non-existent source", func(t *testing.T) {
		renameReq := map[string]any{
			"path":    "/home/nonexistent",
			"newName": "newname",
		}

		renameResp, err := suite.Post("/syscall/"+systemID+"/rename", renameReq)
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = renameResp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, renameResp.StatusCode,
			"Expected 404 Not Found, got %d", renameResp.StatusCode)
	})

	t.Run("returns 409 when target already exists", func(t *testing.T) {
		// Create two directories
		mkdirReq1 := map[string]any{"path": "/home/dir1"}
		mkdirReq2 := map[string]any{"path": "/home/dir2"}
		resp1, err := suite.Post("/syscall/"+systemID+"/mkdir", mkdirReq1)
		require.NoError(t, err)
		_ = resp1.Body.Close()
		resp2, err := suite.Post("/syscall/"+systemID+"/mkdir", mkdirReq2)
		require.NoError(t, err)
		_ = resp2.Body.Close()

		// Try to rename dir1 to dir2 (already exists)
		renameReq := map[string]any{
			"path":    "/home/dir1",
			"newName": "dir2",
		}
		renameResp, err := suite.Post("/syscall/"+systemID+"/rename", renameReq)
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = renameResp.Body.Close() }()

		assert.Equal(t, http.StatusConflict, renameResp.StatusCode,
			"Expected 409 Conflict, got %d", renameResp.StatusCode)
	})

	t.Run("rejects newName with path separator", func(t *testing.T) {
		mkdirReq := map[string]any{"path": "/home/testdir"}
		resp, err := suite.Post("/syscall/"+systemID+"/mkdir", mkdirReq)
		require.NoError(t, err)
		_ = resp.Body.Close()

		// Try to rename with a path instead of just a name
		renameReq := map[string]any{
			"path":    "/home/testdir",
			"newName": "otherdir/subdir", // Invalid: contains path separator
		}
		renameResp, err := suite.Post("/syscall/"+systemID+"/rename", renameReq)
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = renameResp.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, renameResp.StatusCode,
			"Expected 400 Bad Request for newName with path separator")
	})
}

// TestFs_Mv tests file/directory moving
func TestFs_Mv(t *testing.T) {
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

	// Setup user and system via API
	_, systemData, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := systemData["system"].(map[string]any)["id"].(string)

	// Create directories for testing
	mkdirReq1 := map[string]any{"path": "/home/srcdir"}
	mkdirReq2 := map[string]any{"path": "/home/destdir"}
	resp1, err := suite.Post("/syscall/"+systemID+"/mkdir", mkdirReq1)
	require.NoError(t, err)
	_ = resp1.Body.Close()
	resp2, err := suite.Post("/syscall/"+systemID+"/mkdir", mkdirReq2)
	require.NoError(t, err)
	_ = resp2.Body.Close()

	t.Run("moves file to different directory", func(t *testing.T) {
		// Create a subdirectory in srcdir
		subMkdirReq := map[string]any{"path": "/home/srcdir/subdir"}
		subResp, err := suite.Post("/syscall/"+systemID+"/mkdir", subMkdirReq)
		require.NoError(t, err)
		_ = subResp.Body.Close()

		// Move to destdir (keeps name)
		moveReq := map[string]any{
			"sources":     []string{"/home/srcdir/subdir"},
			"destination": "/home/destdir",
		}
		moveResp, err := suite.Post("/syscall/"+systemID+"/mv", moveReq)
		require.NoError(t, err, "Failed to move directory")
		defer func() { _ = moveResp.Body.Close() }()

		require.Equal(t, http.StatusOK, moveResp.StatusCode,
			"Expected 200 OK, got %d", moveResp.StatusCode)

		// Verify old path doesn't exist
		statOldResp, err := suite.Get("/fs/" + systemID + "/stat?path=" + url.QueryEscape("/home/srcdir/subdir"))
		require.NoError(t, err)
		_ = statOldResp.Body.Close()
		assert.Equal(t, http.StatusNotFound, statOldResp.StatusCode,
			"Old path should not exist after move")

		// Verify new path exists
		statNewResp, err := suite.Get("/fs/" + systemID + "/stat?path=" + url.QueryEscape("/home/destdir/subdir"))
		require.NoError(t, err)
		_ = statNewResp.Body.Close()
		assert.Equal(t, http.StatusOK, statNewResp.StatusCode,
			"New path should exist after move")
	})

	t.Run("moves and renames in one operation", func(t *testing.T) {
		// Create another subdirectory
		subMkdirReq := map[string]any{"path": "/home/srcdir/another"}
		subResp, err := suite.Post("/syscall/"+systemID+"/mkdir", subMkdirReq)
		require.NoError(t, err)
		_ = subResp.Body.Close()

		// Move with new name
		moveReq := map[string]any{
			"sources":     []string{"/home/srcdir/another"},
			"destination": "/home/destdir/renamed",
		}
		moveResp, err := suite.Post("/syscall/"+systemID+"/mv", moveReq)
		require.NoError(t, err, "Failed to move and rename")
		defer func() { _ = moveResp.Body.Close() }()

		require.Equal(t, http.StatusOK, moveResp.StatusCode,
			"Expected 200 OK, got %d", moveResp.StatusCode)

		// Verify renamed path exists
		statResp, err := suite.Get("/fs/" + systemID + "/stat?path=" + url.QueryEscape("/home/destdir/renamed"))
		require.NoError(t, err)
		_ = statResp.Body.Close()
		assert.Equal(t, http.StatusOK, statResp.StatusCode,
			"Renamed path should exist after move")
	})

	t.Run("returns errors for non-existent source", func(t *testing.T) {
		moveReq := map[string]any{
			"sources":     []string{"/home/nonexistent"},
			"destination": "/home/destdir",
		}
		moveResp, err := suite.Post("/syscall/"+systemID+"/mv", moveReq)
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = moveResp.Body.Close() }()

		assert.Equal(t, http.StatusOK, moveResp.StatusCode,
			"Expected 200 OK, got %d", moveResp.StatusCode)

		var result map[string]any
		body, err := io.ReadAll(moveResp.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &result))
		errs := result["errors"].([]any)
		assert.NotEmpty(t, errs, "Expected errors for non-existent source")
	})

	t.Run("moves into existing directory (Unix mv behavior)", func(t *testing.T) {
		// Create a directory in destdir
		subMkdirReq := map[string]any{"path": "/home/destdir/existing"}
		subResp, err := suite.Post("/syscall/"+systemID+"/mkdir", subMkdirReq)
		require.NoError(t, err)
		_ = subResp.Body.Close()

		// Move srcdir to an existing directory - should move INTO it
		moveReq := map[string]any{
			"sources":     []string{"/home/srcdir"},
			"destination": "/home/destdir/existing",
		}
		moveResp, err := suite.Post("/syscall/"+systemID+"/mv", moveReq)
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = moveResp.Body.Close() }()

		// Should succeed - srcdir is moved INTO destdir/existing
		assert.Equal(t, http.StatusOK, moveResp.StatusCode,
			"Expected 200 OK for moving into existing directory")

		// Verify the new location exists
		statResp, err := suite.Get("/fs/" + systemID + "/stat?path=" + url.QueryEscape("/home/destdir/existing/srcdir"))
		require.NoError(t, err)
		_ = statResp.Body.Close()
		assert.Equal(t, http.StatusOK, statResp.StatusCode,
			"Source should exist inside destination directory")
	})

	t.Run("returns errors when target name exists in destination", func(t *testing.T) {
		// Create fresh source and destination directories for this test
		srcMkdirReq := map[string]any{"path": "/home/movesrc"}
		srcResp, err := suite.Post("/syscall/"+systemID+"/mkdir", srcMkdirReq)
		require.NoError(t, err)
		_ = srcResp.Body.Close()

		destMkdirReq := map[string]any{"path": "/home/movedest"}
		destResp, err := suite.Post("/syscall/"+systemID+"/mkdir", destMkdirReq)
		require.NoError(t, err)
		_ = destResp.Body.Close()

		// Create a directory named "conflict" in movedest
		conflictMkdirReq := map[string]any{"path": "/home/movedest/conflict"}
		conflictResp, err := suite.Post("/syscall/"+systemID+"/mkdir", conflictMkdirReq)
		require.NoError(t, err)
		_ = conflictResp.Body.Close()

		// Create a directory with same name in movesrc
		sameMkdirReq := map[string]any{"path": "/home/movesrc/conflict"}
		sameResp, err := suite.Post("/syscall/"+systemID+"/mkdir", sameMkdirReq)
		require.NoError(t, err)
		_ = sameResp.Body.Close()

		// Try to move movesrc/conflict to /home/movedest where "conflict" already exists
		moveReq := map[string]any{
			"sources":     []string{"/home/movesrc/conflict"},
			"destination": "/home/movedest",
		}
		moveResp, err := suite.Post("/syscall/"+systemID+"/mv", moveReq)
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = moveResp.Body.Close() }()

		assert.Equal(t, http.StatusOK, moveResp.StatusCode,
			"Expected 200 OK, got %d", moveResp.StatusCode)

		var result map[string]any
		body, err := io.ReadAll(moveResp.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &result))
		errs := result["errors"].([]any)
		assert.NotEmpty(t, errs, "Expected errors for conflicting target")
	})

	t.Run("returns errors for same source and destination", func(t *testing.T) {
		moveReq := map[string]any{
			"sources":     []string{"/home/srcdir"},
			"destination": "/home/srcdir",
		}
		moveResp, err := suite.Post("/syscall/"+systemID+"/mv", moveReq)
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = moveResp.Body.Close() }()

		assert.Equal(t, http.StatusOK, moveResp.StatusCode,
			"Expected 200 OK, got %d", moveResp.StatusCode)

		var result map[string]any
		body, err := io.ReadAll(moveResp.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &result))
		errs := result["errors"].([]any)
		assert.NotEmpty(t, errs, "Expected errors for same source and destination")
	})
}
