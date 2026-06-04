//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gcapp "github.com/mandacode-labs/retrowin-go/internal/application/gc"
	"github.com/mandacode-labs/retrowin-go/internal/core/object"
)

func TestIntegration_GC_PendingCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("cleans expired pending objects", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "gcuser")
		require.NoError(t, err)

		// Create a pending object (initiate but don't complete)
		session, err := suite.ObjectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        100,
		})
		require.NoError(t, err)

		// Backdate the object to make it appear expired
		_, err = suite.DB.ExecContext(ctx,
			"UPDATE objects SET update_time = NOW() - INTERVAL '25 hours' WHERE id = $1",
			session.ObjectID)
		require.NoError(t, err)

		// Run GC with 24h expiry
		gc := gcapp.NewGarbageCollector(suite.ObjectSvc, nil, 0)
		result, err := gc.Run(ctx)
		require.NoError(t, err)

		assert.Equal(t, 1, result.PendingCleaned, "Should clean 1 expired pending object")
		assert.Equal(t, 0, result.OrphansCleaned, "Should have no orphans")

		// Verify object is gone
		_, err = suite.ObjectSvc.GetByID(ctx, session.ObjectID)
		assert.Error(t, err)
	})
}

func TestIntegration_GC_OrphanCleanup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("cleans orphaned active objects", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "gcuser2")
		require.NoError(t, err)

		// Upload and complete a file
		session, err := suite.ObjectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        13,
		})
		require.NoError(t, err)

		data := []byte("test content")
		err = uploadToPresignedURL(session.UploadURL, data)
		require.NoError(t, err)

		obj, err := suite.ObjectSvc.CompleteUpload(ctx, session.ObjectID)
		require.NoError(t, err)

		// Manually delete from S3 to simulate external deletion
		err = suite.MinioClient.RemoveObject(ctx, suite.BucketName(), obj.StorageKey(), minio.RemoveObjectOptions{})
		require.NoError(t, err)

		// Run GC
		gc := gcapp.NewGarbageCollector(suite.ObjectSvc, suite.ObjectStorage, 0)
		result, err := gc.Run(ctx)
		require.NoError(t, err)

		assert.Equal(t, 0, result.PendingCleaned, "Should have no pending objects")
		assert.Equal(t, 1, result.OrphansCleaned, "Should clean 1 orphan")

		// Verify object is gone from DB
		_, err = suite.ObjectSvc.GetByID(ctx, obj.ID())
		assert.Error(t, err)
	})
}

func TestIntegration_GC_NoFalsePositives(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("does not clean healthy objects", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "gcuser3")
		require.NoError(t, err)

		// Upload and complete a file
		session, err := suite.ObjectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        13,
		})
		require.NoError(t, err)

		data := []byte("test content")
		err = uploadToPresignedURL(session.UploadURL, data)
		require.NoError(t, err)

		obj, err := suite.ObjectSvc.CompleteUpload(ctx, session.ObjectID)
		require.NoError(t, err)

		// Run GC — object should still be in S3, so not cleaned
		gc := gcapp.NewGarbageCollector(suite.ObjectSvc, suite.ObjectStorage, 0)
		result, err := gc.Run(ctx)
		require.NoError(t, err)

		assert.Equal(t, 0, result.PendingCleaned)
		assert.Equal(t, 0, result.OrphansCleaned, "Healthy objects should not be cleaned")

		// Verify object still exists
		existing, err := suite.ObjectSvc.GetByID(ctx, obj.ID())
		require.NoError(t, err)
		assert.Equal(t, obj.ID(), existing.ID())
	})
}
