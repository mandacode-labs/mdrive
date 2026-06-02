//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/retrowin-go/internal/core/object"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func TestIntegration_ObjectService_InitiateUpload(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("creates pending object with presigned URL", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser")
		require.NoError(t, err)

		session, err := suite.ObjectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        1024,
		})

		require.NoError(t, err)
		assert.NotEmpty(t, session.ObjectID)
		assert.NotEmpty(t, session.UploadURL)
		assert.True(t, session.ExpiresAt.After(time.Now()))

		// Verify object exists in DB as pending
		obj, err := suite.ObjectSvc.GetByID(ctx, session.ObjectID)
		require.NoError(t, err)
		assert.Equal(t, object.StatusPending, obj.Status())
		assert.Equal(t, sys.ID, obj.SystemID())
	})

	t.Run("returns same session for duplicate idempotency key", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser2")
		require.NoError(t, err)

		idempotencyKey := "test-idempotency-key-123"
		checksum := "dGVzdA=="

		session1, err := suite.ObjectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
			SystemID:       sys.ID,
			ContentType:    "text/plain",
			Size:           1024,
			Checksum:       &checksum,
			IdempotencyKey: &idempotencyKey,
		})
		require.NoError(t, err)

		session2, err := suite.ObjectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
			SystemID:       sys.ID,
			ContentType:    "text/plain",
			Size:           1024,
			Checksum:       &checksum,
			IdempotencyKey: &idempotencyKey,
		})
		require.NoError(t, err)

		assert.Equal(t, session1.ObjectID, session2.ObjectID, "Should return same object ID")
	})

	t.Run("rejects initiate without system ID", func(t *testing.T) {
		_, err := suite.ObjectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
			Size: 1024,
		})
		assert.Error(t, err)
	})
}

func TestIntegration_ObjectService_CompleteUpload(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("completes upload and verifies object exists in storage", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser")
		require.NoError(t, err)

		// Initiate upload
		session, err := suite.ObjectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        13,
		})
		require.NoError(t, err)

		// Upload actual content to presigned URL
		data := []byte("test content")
		err = uploadToPresignedURL(session.UploadURL, data)
		require.NoError(t, err, "Failed to upload to presigned URL")

		// Complete upload
		obj, err := suite.ObjectSvc.CompleteUpload(ctx, session.ObjectID)
		require.NoError(t, err)
		assert.Equal(t, object.StatusActive, obj.Status())

		// Verify object exists in storage
		exists, err := suite.ObjectStorage.ObjectExists(ctx, suite.BucketName(), obj.StorageKey())
		require.NoError(t, err)
		assert.True(t, exists, "Object should exist in storage")

		// Verify size matches
		size, err := suite.ObjectSvc.GetObjectSize(ctx, obj.ID())
		require.NoError(t, err)
		assert.Equal(t, int64(len(data)), size)
	})

	t.Run("rejects completion for non-existent object", func(t *testing.T) {
		_, err := suite.ObjectSvc.CompleteUpload(ctx, "non-existent-id")
		assert.Error(t, err)
		assert.True(t, errors.IsNotFound(err))
	})

	t.Run("rejects completion when object not uploaded to storage", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser3")
		require.NoError(t, err)

		session, err := suite.ObjectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        100,
		})
		require.NoError(t, err)

		// Don't upload anything — object should not be in storage
		_, err = suite.ObjectSvc.CompleteUpload(ctx, session.ObjectID)
		assert.Error(t, err)
	})
}

func TestIntegration_ObjectService_ChecksumValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("rejects completion with checksum mismatch", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser")
		require.NoError(t, err)

		// Calculate checksum for "wrong content"
		wrongData := []byte("wrong content")
		wrongHash := md5.Sum(wrongData)
		wrongChecksum := base64.StdEncoding.EncodeToString(wrongHash[:])

		// Initiate with wrong checksum
		session, err := suite.ObjectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        int64(len(wrongData)),
			Checksum:    &wrongChecksum,
		})
		require.NoError(t, err)

		// Upload DIFFERENT content (correct data)
		correctData := []byte("correct content")
		err = uploadToPresignedURL(session.UploadURL, correctData)
		require.NoError(t, err)

		// Complete should fail with checksum mismatch
		_, err = suite.ObjectSvc.CompleteUpload(ctx, session.ObjectID)
		assert.Error(t, err)
	})
}

func TestIntegration_ObjectService_Delete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("deletes object from both storage and DB", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser")
		require.NoError(t, err)

		// Create and complete an upload
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

		// Delete the object
		err = suite.ObjectSvc.Delete(ctx, obj.ID())
		require.NoError(t, err)

		// Verify object no longer exists
		_, err = suite.ObjectSvc.GetByID(ctx, obj.ID())
		assert.Error(t, err)
		assert.True(t, errors.IsNotFound(err))
	})
}

// uploadToPresignedURL uploads data to a presigned URL for testing.
func uploadToPresignedURL(presignedURL string, data []byte) error {
	req, err := http.NewRequest("PUT", presignedURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("upload failed with status %d", resp.StatusCode)
	}
	return nil
}
