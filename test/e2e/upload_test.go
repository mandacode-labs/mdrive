package e2e

import (
	"context"
	// #nosec G401 - MD5 is used for S3 ETag checksum testing, not security
	"crypto/md5"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpload_Initiate tests upload session initiation
func TestUpload_Initiate(t *testing.T) {
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
	env, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	t.Run("initiates upload for new file", func(t *testing.T) {
		req := map[string]any{
			"path":        "/home/uploaded.txt",
			"contentType": "text/plain",
			"size":        int64(1024),
		}

		resp, err := suite.Post("/fs/"+systemID+"/upload/initiate", req)
		require.NoError(t, err, "Failed to initiate upload")
		defer func() { _ = resp.Body.Close() }()

		var result map[string]any
		err = suite.ReadJSON(resp, &result)
		require.NoError(t, err, "Failed to read response JSON")

		require.Equal(t, http.StatusCreated, resp.StatusCode,
			"Expected 201 Created, got %d: %v", resp.StatusCode, result)

		// Check uploadSession object
		session, ok := result["uploadSession"].(map[string]any)
		require.True(t, ok, "Response should contain uploadSession object, got: %v", result)

		// Should return object ID and upload URL
		assert.NotEmpty(t, session["objectId"], "Should have objectId")
		assert.NotEmpty(t, session["uploadUrl"], "Should have uploadUrl")
		assert.NotEmpty(t, session["expiresAt"], "Should have expiresAt")
	})

	t.Run("rejects upload for invalid path", func(t *testing.T) {
		req := map[string]any{
			"path":        "invalid-no-leading-slash",
			"contentType": "text/plain",
			"size":        int64(1024),
		}

		resp, err := suite.Post("/fs/"+systemID+"/upload/initiate", req)
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
			"Expected 400 Bad Request for invalid path, got %d", resp.StatusCode)
	})

	t.Run("rejects upload without authentication", func(t *testing.T) {
		suite.ClearCookies()

		req := map[string]any{
			"path":        "/home/unauthorized.txt",
			"contentType": "text/plain",
			"size":        int64(1024),
		}

		resp, err := suite.Post("/fs/"+systemID+"/upload/initiate", req)
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"Expected 401 Unauthorized, got %d", resp.StatusCode)
	})
}

// TestUpload_Complete tests upload completion
func TestUpload_Complete(t *testing.T) {
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
	env, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	t.Run("completes upload and creates inode", func(t *testing.T) {
		objectID := suite.InitiateUpload(t, systemID, "/home/completed.txt")

		completeReq := map[string]any{
			"objectId": objectID,
			"path":     "/home/completed.txt",
			"mode":     0644,
		}

		completeResp, err := suite.Post("/fs/"+systemID+"/upload/complete", completeReq)
		require.NoError(t, err, "Failed to complete upload")
		defer func() { _ = completeResp.Body.Close() }()

		require.Equal(t, http.StatusCreated, completeResp.StatusCode,
			"Expected 201 Created, got %d", completeResp.StatusCode)
	})

	t.Run("rejects completion with invalid object ID", func(t *testing.T) {
		req := map[string]any{
			"objectId": "nonexistent-object-id",
			"path":     "/home/invalid.txt",
			"mode":     0644,
		}

		resp, err := suite.Post("/fs/"+systemID+"/upload/complete", req)
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"Expected 404 Not Found for invalid object ID, got %d", resp.StatusCode)
	})

	t.Run("applies custom permissions", func(t *testing.T) {
		objectID := suite.InitiateUpload(t, systemID, "/home/custom-perms.txt")

		// Complete with custom permissions
		completeReq := map[string]any{
			"objectId": objectID,
			"path":     "/home/custom-perms.txt",
			"mode":     0600,
		}

		completeResp, err := suite.Post("/fs/"+systemID+"/upload/complete", completeReq)
		require.NoError(t, err)
		defer func() { _ = completeResp.Body.Close() }()

		require.Equal(t, http.StatusCreated, completeResp.StatusCode,
			"Expected 201 Created, got %d", completeResp.StatusCode)
	})
}

// TestUpload_Download tests download URL generation
func TestUpload_Download(t *testing.T) {
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
	env, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	t.Run("returns 404 for non-existent file", func(t *testing.T) {
		resp, err := suite.Get("/fs/" + systemID + "/download?path=" + url.QueryEscape("/home/nonexistent.txt"))
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"Expected 404 Not Found, got %d", resp.StatusCode)
	})

	t.Run("rejects download for directory", func(t *testing.T) {
		resp, err := suite.Get("/fs/" + systemID + "/download?path=" + url.QueryEscape("/home"))
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
			"Expected 400 Bad Request for directory, got %d", resp.StatusCode)
	})
}

// TestUpload_FullFlow tests the complete upload flow
func TestUpload_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	err = suite.StartServer(ctx)
	require.NoError(t, err, "Failed to start server")

	// Setup user and system via API
	env, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	t.Run("upload and download cycle", func(t *testing.T) {
		objectID, uploadURL := suite.InitiateUploadWithURL(t, systemID, "/home/cycle-test.txt")

		assert.NotEmpty(t, objectID, "Should have objectId")
		assert.NotEmpty(t, uploadURL, "Should have uploadUrl")

		// Complete upload
		completeReq := map[string]any{
			"objectId": objectID,
			"path":     "/home/cycle-test.txt",
			"mode":     0644,
		}
		completeResp, err := suite.Post("/fs/"+systemID+"/upload/complete", completeReq)
		require.NoError(t, err, "Failed to complete upload")
		defer func() { _ = completeResp.Body.Close() }()

		require.Equal(t, http.StatusCreated, completeResp.StatusCode,
			"Expected 201 Created, got %d", completeResp.StatusCode)

		// Get download URL
		downloadResp, err := suite.Get("/fs/" + systemID + "/download?path=" + url.QueryEscape("/home/cycle-test.txt"))
		require.NoError(t, err, "Failed to get download URL")
		defer func() { _ = downloadResp.Body.Close() }()

		require.Equal(t, http.StatusOK, downloadResp.StatusCode,
			"Expected 200 OK, got %d", downloadResp.StatusCode)
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		objectID1, _ := suite.InitiateUploadWithURL(t, systemID, "/home/overwrite.txt")

		completeReq1 := map[string]any{
			"objectId": objectID1,
			"path":     "/home/overwrite.txt",
			"mode":     0644,
		}
		completeResp1, err := suite.Post("/fs/"+systemID+"/upload/complete", completeReq1)
		require.NoError(t, err)
		defer func() { _ = completeResp1.Body.Close() }()
		require.Equal(t, http.StatusCreated, completeResp1.StatusCode)

		// Upload second version (same path)
		objectID2, _ := suite.InitiateUploadWithURL(t, systemID, "/home/overwrite.txt")

		// Second object should be different
		assert.NotEqual(t, objectID1, objectID2,
			"Different uploads should have different objectIds")

		// Complete second version
		completeReq2 := map[string]any{
			"objectId": objectID2,
			"path":     "/home/overwrite.txt",
			"mode":     0644,
		}
		completeResp2, err := suite.Post("/fs/"+systemID+"/upload/complete", completeReq2)
		require.NoError(t, err)
		defer func() { _ = completeResp2.Body.Close() }()
		if completeResp2.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(completeResp2.Body)
			t.Fatalf("Expected 201, got %d: %s", completeResp2.StatusCode, string(body))
		}
	})
}

// TestUpload_ChecksumMismatch verifies that upload completion is rejected when content does not match declared checksum.
func TestUpload_ChecksumMismatch(t *testing.T) {
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

	env, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	// Initiate with checksum for "wrong content"
	wrongData := []byte("wrong content")
	// #nosec G401 - MD5 used for S3 ETag checksum testing
	wrongHash := md5.Sum(wrongData)
	wrongChecksum := base64.StdEncoding.EncodeToString(wrongHash[:])

	req := map[string]any{
		"path":        "/home/mismatch.txt",
		"contentType": "text/plain",
		"size":        int64(len(wrongData)),
		"checksum":    wrongChecksum,
	}
	resp, err := suite.Post("/fs/"+systemID+"/upload/initiate", req)
	require.NoError(t, err, "Failed to initiate upload")
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "Initiate should succeed")

	var result map[string]any
	err = suite.ReadJSON(resp, &result)
	require.NoError(t, err, "Failed to read response JSON")

	session := result["uploadSession"].(map[string]any)
	objectID := session["objectId"].(string)
	uploadURL := session["uploadUrl"].(string)

	// Upload DIFFERENT content (correct data, not wrongData)
	correctData := []byte("correct content")
	suite.UploadToPresignedURL(t, uploadURL, correctData)

	// Complete should fail with checksum mismatch
	completeReq := map[string]any{
		"objectId": objectID,
		"path":     "/home/mismatch.txt",
		"mode":     0644,
	}
	completeResp, err := suite.Post("/fs/"+systemID+"/upload/complete", completeReq)
	require.NoError(t, err, "Failed to complete upload")
	defer func() { _ = completeResp.Body.Close() }()

	assert.Equal(t, http.StatusBadRequest, completeResp.StatusCode,
		"Expected 400 Bad Request for checksum mismatch, got %d", completeResp.StatusCode)
}

// TestUpload_Idempotency verifies that duplicate initiation requests with the same idempotency key return the same session.
func TestUpload_Idempotency(t *testing.T) {
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

	env, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	idempotencyKey := "test-idempotency-key-123"
	data := []byte("idempotency test content")
	// #nosec G401 - MD5 used for S3 ETag checksum testing
	hash := md5.Sum(data)
	checksum := base64.StdEncoding.EncodeToString(hash[:])

	req := map[string]any{
		"path":           "/home/idempotent.txt",
		"contentType":    "text/plain",
		"size":           int64(len(data)),
		"checksum":       checksum,
		"idempotencyKey": idempotencyKey,
	}

	// First request
	resp1, err := suite.Post("/fs/"+systemID+"/upload/initiate", req)
	require.NoError(t, err, "Failed to initiate upload (1st)")
	defer func() { _ = resp1.Body.Close() }()
	require.Equal(t, http.StatusCreated, resp1.StatusCode, "First initiate should succeed")

	var result1 map[string]any
	err = suite.ReadJSON(resp1, &result1)
	require.NoError(t, err, "Failed to read response JSON")

	session1 := result1["uploadSession"].(map[string]any)
	objectID1 := session1["objectId"].(string)

	// Second request with same idempotency key
	resp2, err := suite.Post("/fs/"+systemID+"/upload/initiate", req)
	require.NoError(t, err, "Failed to initiate upload (2nd)")
	defer func() { _ = resp2.Body.Close() }()
	require.Equal(t, http.StatusCreated, resp2.StatusCode, "Second initiate should succeed")

	var result2 map[string]any
	err = suite.ReadJSON(resp2, &result2)
	require.NoError(t, err, "Failed to read response JSON")

	session2 := result2["uploadSession"].(map[string]any)
	objectID2 := session2["objectId"].(string)

	// Should return the same object ID
	assert.Equal(t, objectID1, objectID2,
		"Idempotent requests should return the same objectId")

	// Upload and complete
	uploadURL := session2["uploadUrl"].(string)
	suite.UploadToPresignedURL(t, uploadURL, data)

	completeReq := map[string]any{
		"objectId": objectID2,
		"path":     "/home/idempotent.txt",
		"mode":     0644,
	}
	completeResp, err := suite.Post("/fs/"+systemID+"/upload/complete", completeReq)
	require.NoError(t, err, "Failed to complete upload")
	defer func() { _ = completeResp.Body.Close() }()
	require.Equal(t, http.StatusCreated, completeResp.StatusCode, "Complete should succeed")
}
