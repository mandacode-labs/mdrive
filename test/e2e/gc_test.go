package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gcapp "github.com/mandacode-labs/retrowin-go/internal/application/gc"
	objectdomain "github.com/mandacode-labs/retrowin-go/internal/core/object"
	objectrepo "github.com/mandacode-labs/retrowin-go/internal/core/object/repository"
	s3storage "github.com/mandacode-labs/retrowin-go/internal/core/object/s3"
)

func TestGC_PendingCleanup(t *testing.T) {
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

	env, err := suite.SetupFullEnvironmentAPI(ctx, "gcuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	// Initiate upload but don't actually upload to S3 — leaves object in pending state
	resp, err := suite.Post("/fs/"+systemID+"/upload/initiate", map[string]any{
		"path":        "/home/pending-file.txt",
		"contentType": "text/plain",
		"size":        100,
	})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, 201, resp.StatusCode)

	// Verify the pending object exists in DB
	db := suite.GetDB()
	rows, err := db.QueryContext(ctx, "SELECT id FROM objects WHERE system_id = $1 AND status = 'pending'", systemID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var objectID string
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(&objectID))

	// Backdate the pending object so it appears expired (older than 24h default expiry)
	_, err = db.ExecContext(ctx, "UPDATE objects SET update_time = NOW() - INTERVAL '25 hours' WHERE id = $1", objectID)
	require.NoError(t, err, "Failed to backdate pending object")

	// Build GC dependencies (same as cmd/gc)
	cfg := suite.GetConfig()
	objStorage, err := s3storage.New(&cfg.Storage)
	require.NoError(t, err, "Failed to create S3 storage")

	objectSvc := objectdomain.NewService(objectrepo.NewRepository(suite.GetEntClient()), objStorage)

	// Run GC with default expiry (24h) — our backdated object should be cleaned
	gc := gcapp.NewGarbageCollector(objectSvc, objStorage, 0)
	result, err := gc.Run(ctx)
	require.NoError(t, err, "GC run failed")

	assert.Equal(t, 1, result.PendingCleaned, "Should have cleaned 1 pending object")
	assert.Equal(t, 0, result.OrphansCleaned, "Should have no orphans")

	// Verify the pending object is gone from DB
	var remaining int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM objects WHERE system_id = $1 AND status = 'pending'", systemID).Scan(&remaining)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining, "Pending object should be removed from DB")
}

func TestGC_OrphanCleanup(t *testing.T) {
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

	env, err := suite.SetupFullEnvironmentAPI(ctx, "gcuser2")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	// Upload a file via the full flow (initiate + upload to S3 + complete)
	resp, err := suite.Post("/fs/"+systemID+"/upload/initiate", map[string]any{
		"path":        "/home/orphan-file.txt",
		"contentType": "text/plain",
		"size":        13,
	})
	require.NoError(t, err)

	var initResult map[string]any
	err = suite.ReadJSON(resp, &initResult)
	require.NoError(t, err)

	session := initResult["uploadSession"].(map[string]any)
	objectID := session["objectId"].(string)
	uploadURL := session["uploadUrl"].(string)

	// Upload actual data to S3
	suite.UploadToPresignedURL(t, uploadURL, []byte("test content"))

	// Complete upload
	resp2, err := suite.Post("/fs/"+systemID+"/upload/complete", map[string]any{
		"objectId": objectID,
		"path":     "/home/orphan-file.txt",
	})
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()
	require.Equal(t, 201, resp2.StatusCode)

	// Get the object's storage key from DB to delete from S3 directly
	db := suite.GetDB()
	var bucket, storageKey string
	err = db.QueryRowContext(ctx, "SELECT bucket, storage_key FROM objects WHERE id = $1", objectID).Scan(&bucket, &storageKey)
	require.NoError(t, err)

	// Manually delete the object from S3 (simulating external deletion)
	minioClient, err := minio.New(suite.MinioAddr, &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	require.NoError(t, err)

	err = minioClient.RemoveObject(ctx, bucket, storageKey, minio.RemoveObjectOptions{})
	require.NoError(t, err, "Failed to delete object from MinIO")

	// Build GC dependencies
	cfg := suite.GetConfig()
	objStorage, err := s3storage.New(&cfg.Storage)
	require.NoError(t, err, "Failed to create S3 storage")

	objectSvc := objectdomain.NewService(objectrepo.NewRepository(suite.GetEntClient()), objStorage)

	// Run GC
	gc := gcapp.NewGarbageCollector(objectSvc, objStorage, 0)
	result, err := gc.Run(ctx)
	require.NoError(t, err, "GC run failed")

	assert.Equal(t, 0, result.PendingCleaned, "Should have no pending objects")
	assert.Equal(t, 1, result.OrphansCleaned, "Should have cleaned 1 orphan")

	// Verify the orphaned object is gone from DB
	var remaining int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM objects WHERE id = $1", objectID).Scan(&remaining)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining, "Orphaned object should be removed from DB")
}

func TestGC_NoOrphans(t *testing.T) {
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

	env, err := suite.SetupFullEnvironmentAPI(ctx, "gcuser3")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	// Upload a file via the full flow — this should NOT be cleaned by GC
	resp, err := suite.Post("/fs/"+systemID+"/upload/initiate", map[string]any{
		"path":        "/home/healthy-file.txt",
		"contentType": "text/plain",
		"size":        13,
	})
	require.NoError(t, err)

	var initResult map[string]any
	err = suite.ReadJSON(resp, &initResult)
	require.NoError(t, err)

	session := initResult["uploadSession"].(map[string]any)
	objectID := session["objectId"].(string)
	uploadURL := session["uploadUrl"].(string)

	// Upload actual data to S3
	suite.UploadToPresignedURL(t, uploadURL, []byte("test content"))

	// Complete upload
	resp2, err := suite.Post("/fs/"+systemID+"/upload/complete", map[string]any{
		"objectId": objectID,
		"path":     "/home/healthy-file.txt",
	})
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()
	require.Equal(t, 201, resp2.StatusCode)

	// Build GC dependencies
	cfg := suite.GetConfig()
	objStorage, err := s3storage.New(&cfg.Storage)
	require.NoError(t, err)

	objectSvc := objectdomain.NewService(objectrepo.NewRepository(suite.GetEntClient()), objStorage)

	// Run GC
	gc := gcapp.NewGarbageCollector(objectSvc, objStorage, 0)
	result, err := gc.Run(ctx)
	require.NoError(t, err, "GC run failed")

	assert.Equal(t, 0, result.PendingCleaned, "Should have no pending objects")
	assert.Equal(t, 0, result.OrphansCleaned, "Healthy objects should not be cleaned")

	// Verify the object still exists in DB
	db := suite.GetDB()
	var remaining int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM objects WHERE id = $1", objectID).Scan(&remaining)
	require.NoError(t, err)
	assert.Equal(t, 1, remaining, "Healthy object should still exist in DB")
}
