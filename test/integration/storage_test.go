//go:build integration

package integration

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/retrowin-go/internal/application/storage"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
)

func TestIntegration_StorageService_UploadFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("initiates upload session", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser")
		require.NoError(t, err)

		session, err := suite.StorageSvc.InitiateUpload(ctx, &storage.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        1024,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, session.ObjectID)
		assert.NotEmpty(t, session.UploadURL)
	})

	t.Run("completes upload and creates inode", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser2")
		require.NoError(t, err)

		// Initiate
		session, err := suite.StorageSvc.InitiateUpload(ctx, &storage.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        13,
		})
		require.NoError(t, err)

		// Upload
		data := []byte("test content")
		err = uploadToPresignedURL(session.UploadURL, data)
		require.NoError(t, err)

		// Complete
		result, err := suite.StorageSvc.CompleteUpload(ctx, &storage.CompleteUploadCommand{
			ObjectID: session.ObjectID,
			SystemID: sys.ID,
			Mode:     inode.ModeObject | 0644,
		})
		require.NoError(t, err)
		assert.NotNil(t, result.Inode)
		assert.Equal(t, int64(len(data)), result.Inode.Size())
		assert.Equal(t, inode.ModeObject|0644, result.Inode.Mode())
	})

	t.Run("generates download URL", func(t *testing.T) {
		u, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser3")
		require.NoError(t, err)

		authCtx := suite.AuthenticatedContext(ctx, u.ID)

		// Upload and complete
		session, err := suite.StorageSvc.InitiateUpload(authCtx, &storage.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        13,
		})
		require.NoError(t, err)

		data := []byte("test content")
		err = uploadToPresignedURL(session.UploadURL, data)
		require.NoError(t, err)

		result, err := suite.StorageSvc.CompleteUpload(authCtx, &storage.CompleteUploadCommand{
			ObjectID: session.ObjectID,
			SystemID: sys.ID,
			Mode:     inode.ModeObject | 0644,
		})
		require.NoError(t, err)

		// Get download URL
		downloadURL, expiresAt, err := suite.StorageSvc.GetDownloadURL(authCtx, result.Inode.ID())
		require.NoError(t, err)
		assert.NotEmpty(t, downloadURL)
		assert.True(t, expiresAt.After(time.Now()))
	})
}

func TestIntegration_StorageService_ChecksumAndIdempotency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("upload with checksum validation", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser")
		require.NoError(t, err)

		data := []byte("test content")
		hash := md5.Sum(data)
		checksum := base64.StdEncoding.EncodeToString(hash[:])

		// Initiate with checksum
		session, err := suite.StorageSvc.InitiateUpload(ctx, &storage.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        int64(len(data)),
			Checksum:    &checksum,
		})
		require.NoError(t, err)

		// Upload matching content
		err = uploadToPresignedURL(session.UploadURL, data)
		require.NoError(t, err)

		// Should complete successfully
		_, err = suite.StorageSvc.CompleteUpload(ctx, &storage.CompleteUploadCommand{
			ObjectID: session.ObjectID,
			SystemID: sys.ID,
			Mode:     inode.ModeObject | 0644,
		})
		require.NoError(t, err)
	})

	t.Run("idempotent upload initiation", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser2")
		require.NoError(t, err)

		idempotencyKey := "integration-test-key-456"
		data := []byte("idempotency test")
		hash := md5.Sum(data)
		checksum := base64.StdEncoding.EncodeToString(hash[:])

		// First request
		session1, err := suite.StorageSvc.InitiateUpload(ctx, &storage.InitiateUploadCommand{
			SystemID:       sys.ID,
			ContentType:    "text/plain",
			Size:           int64(len(data)),
			Checksum:       &checksum,
			IdempotencyKey: &idempotencyKey,
		})
		require.NoError(t, err)

		// Second request with same key
		session2, err := suite.StorageSvc.InitiateUpload(ctx, &storage.InitiateUploadCommand{
			SystemID:       sys.ID,
			ContentType:    "text/plain",
			Size:           int64(len(data)),
			Checksum:       &checksum,
			IdempotencyKey: &idempotencyKey,
		})
		require.NoError(t, err)

		assert.Equal(t, session1.ObjectID, session2.ObjectID)
	})
}
