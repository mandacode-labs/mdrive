package e2e

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseExpiryFromUploadResponse extracts the expiresAt from an upload initiate response.
func parseExpiryFromUploadResponse(t *testing.T, result map[string]any) time.Time {
	t.Helper()

	session, ok := result["uploadSession"].(map[string]any)
	require.True(t, ok, "Response should contain uploadSession, got: %v", result)

	raw, ok := session["expiresAt"]
	require.True(t, ok, "uploadSession should contain expiresAt")

	return parseTimestampValue(t, raw)
}

// parseExpiryFromDownloadResponse extracts the expiresAt from a download URL response.
func parseExpiryFromDownloadResponse(t *testing.T, result map[string]any) time.Time {
	t.Helper()

	dl, ok := result["downloadUrl"].(map[string]any)
	require.True(t, ok, "Response should contain downloadUrl, got: %v", result)

	raw, ok := dl["expiresAt"]
	require.True(t, ok, "downloadUrl should contain expiresAt")

	return parseTimestampValue(t, raw)
}

func parseTimestampValue(t *testing.T, raw any) time.Time {
	t.Helper()

	switch v := raw.(type) {
	case string:
		// ogen serializes Timestamp (date-time) as RFC3339 string
		ts, err := time.Parse(time.RFC3339, v)
		require.NoError(t, err, "Failed to parse timestamp %q", v)
		return ts
	case float64:
		return time.Unix(int64(v), 0)
	default:
		t.Fatalf("unexpected timestamp type %T: %v", raw, raw)
		return time.Time{}
	}
}

// initiateUploadWithExpiry initiates an upload and returns the expiresAt timestamp.
func initiateUploadWithExpiry(t *testing.T, suite *Suite, systemID, filePath string, size int64) time.Time {
	t.Helper()

	req := map[string]any{
		"path":        filePath,
		"contentType": "application/octet-stream",
		"size":        size,
	}

	resp, err := suite.Post("/fs/"+systemID+"/upload/initiate", req)
	require.NoError(t, err, "Failed to initiate upload")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "Initiate should return 201")

	var result map[string]any
	require.NoError(t, suite.ReadJSON(resp, &result), "Failed to read response JSON")

	return parseExpiryFromUploadResponse(t, result)
}

// TestUpload_ExpiryBasedOnSize verifies that presigned upload URL expiry scales with file size.
func TestUpload_ExpiryBasedOnSize(t *testing.T) {
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

	env, err := suite.SetupFullEnvironmentAPI(ctx, "expiryuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	t.Run("small file gets ~15min expiry", func(t *testing.T) {
		before := time.Now()
		expiresAt := initiateUploadWithExpiry(t, suite, systemID, "/home/small.bin", 1*1024*1024)
		after := time.Now()

		ttl := expiresAt.Sub(before)

		assert.GreaterOrEqual(t, ttl, 14*time.Minute,
			"1MB file should have ~15min expiry, got TTL=%v", ttl)
		assert.LessOrEqual(t, ttl, 16*time.Minute,
			"1MB file should have ~15min expiry, got TTL=%v", ttl)
		assert.True(t, expiresAt.After(after), "expiresAt should be in the future")
	})

	t.Run("medium file gets ~1hr expiry", func(t *testing.T) {
		before := time.Now()
		expiresAt := initiateUploadWithExpiry(t, suite, systemID, "/home/medium.bin", 50*1024*1024)

		ttl := expiresAt.Sub(before)

		assert.GreaterOrEqual(t, ttl, 59*time.Minute,
			"50MB file should have ~1hr expiry, got TTL=%v", ttl)
		assert.LessOrEqual(t, ttl, 61*time.Minute,
			"50MB file should have ~1hr expiry, got TTL=%v", ttl)
	})

	t.Run("expiry increases with file size", func(t *testing.T) {
		smallExpiry := initiateUploadWithExpiry(t, suite, systemID, "/home/tier-small.bin", 1*1024*1024)
		mediumExpiry := initiateUploadWithExpiry(t, suite, systemID, "/home/tier-medium.bin", 50*1024*1024)
		largeExpiry := initiateUploadWithExpiry(t, suite, systemID, "/home/tier-large.bin", 500*1024*1024)
		hugeExpiry := initiateUploadWithExpiry(t, suite, systemID, "/home/tier-huge.bin", 5*1024*1024*1024)

		assert.True(t, mediumExpiry.After(smallExpiry),
			"50MB (%v) should expire later than 1MB (%v)", mediumExpiry, smallExpiry)
		assert.True(t, largeExpiry.After(mediumExpiry),
			"500MB (%v) should expire later than 50MB (%v)", largeExpiry, mediumExpiry)
		assert.True(t, hugeExpiry.After(largeExpiry),
			"5GB (%v) should expire later than 500MB (%v)", hugeExpiry, largeExpiry)
	})
}

// TestUpload_DownloadExpiryBasedOnSize verifies that download URL expiry scales with file size.
func TestUpload_DownloadExpiryBasedOnSize(t *testing.T) {
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

	env, err := suite.SetupFullEnvironmentAPI(ctx, "dluser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	// uploadAndDownload uploads a file and returns the download URL expiry
	uploadAndDownload := func(t *testing.T, filePath string, size int64) time.Time {
		t.Helper()

		// Initiate
		initReq := map[string]any{
			"path":        filePath,
			"contentType": "application/octet-stream",
			"size":        size,
		}
		initResp, err := suite.Post("/fs/"+systemID+"/upload/initiate", initReq)
		require.NoError(t, err)

		var initResult map[string]any
		require.NoError(t, suite.ReadJSON(initResp, &initResult))
		require.Equal(t, http.StatusCreated, initResp.StatusCode)

		session := initResult["uploadSession"].(map[string]any)
		objectID := session["objectId"].(string)
		uploadURL := session["uploadUrl"].(string)

		// Upload to MinIO
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}
		suite.UploadToPresignedURL(t, uploadURL, data)

		// Complete
		completeReq := map[string]any{
			"objectId": objectID,
			"path":     filePath,
			"mode":     0644,
		}
		completeResp, err := suite.Post("/fs/"+systemID+"/upload/complete", completeReq)
		require.NoError(t, err)
		_ = completeResp.Body.Close()
		require.Equal(t, http.StatusCreated, completeResp.StatusCode)

		// Download
		dlResp, err := suite.Get("/fs/" + systemID + "/download?path=" + url.QueryEscape(filePath))
		require.NoError(t, err)

		var dlResult map[string]any
		require.NoError(t, suite.ReadJSON(dlResp, &dlResult))
		require.Equal(t, http.StatusOK, dlResp.StatusCode)

		return parseExpiryFromDownloadResponse(t, dlResult)
	}

	t.Run("small file download has ~15min expiry", func(t *testing.T) {
		before := time.Now()
		expiresAt := uploadAndDownload(t, "/home/dl-small.bin", 1024) // 1KB
		after := time.Now()

		ttl := expiresAt.Sub(before)
		assert.GreaterOrEqual(t, ttl, 14*time.Minute,
			"small file download should have ~15min expiry, got TTL=%v", ttl)
		assert.LessOrEqual(t, ttl, 16*time.Minute,
			"small file download should have ~15min expiry, got TTL=%v", ttl)
		assert.True(t, expiresAt.After(after), "expiresAt should be in the future")
	})

	t.Run("large file download has longer expiry than small", func(t *testing.T) {
		smallExpiry := uploadAndDownload(t, "/home/dl-tiny.bin", 1024)         // 1KB -> 15min
		largeExpiry := uploadAndDownload(t, "/home/dl-big.bin", 200*1024*1024) // 200MB -> 3hr

		assert.True(t, largeExpiry.After(smallExpiry),
			"200MB download (%v) should expire later than 1KB download (%v)",
			largeExpiry, smallExpiry)
	})
}
