//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/application/vfs"
	"github.com/mandacode-labs/mdrive/internal/core/inode"
	"github.com/mandacode-labs/mdrive/internal/core/object"
)

func TestIntegration_FsService_CreateDirectory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("creates directory with default permissions", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser")
		require.NoError(t, err)

		dir, err := suite.FsSvc.Mkdir(ctx, sys.ID, "/home/newdir", 0755)
		require.NoError(t, err)
		assert.Equal(t, inode.ModeDirectory|0755, dir.Mode())
		assert.Equal(t, sys.ID, dir.SystemID())

		// Verify directory can be resolved by path
		resolved, err := suite.FsSvc.ResolvePath(ctx, sys.ID, "/home/newdir")
		require.NoError(t, err)
		assert.Equal(t, dir.ID(), resolved.ID())
	})

	t.Run("creates nested directory", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser2")
		require.NoError(t, err)

		// Create parent first
		_, err = suite.FsSvc.Mkdir(ctx, sys.ID, "/home/parent", 0755)
		require.NoError(t, err)

		dir, err := suite.FsSvc.Mkdir(ctx, sys.ID, "/home/parent/child", 0755)
		require.NoError(t, err)
		assert.Equal(t, inode.ModeDirectory|0755, dir.Mode())

		// Verify both parent and child exist
		parent, err := suite.FsSvc.ResolvePath(ctx, sys.ID, "/home/parent")
		require.NoError(t, err)
		assert.Equal(t, inode.ModeDirectory|0755, parent.Mode())

		child, err := suite.FsSvc.ResolvePath(ctx, sys.ID, "/home/parent/child")
		require.NoError(t, err)
		assert.Equal(t, dir.ID(), child.ID())
	})

	t.Run("rejects duplicate directory", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser3")
		require.NoError(t, err)

		_, err = suite.FsSvc.Mkdir(ctx, sys.ID, "/home/duplicate", 0755)
		require.NoError(t, err)

		_, err = suite.FsSvc.Mkdir(ctx, sys.ID, "/home/duplicate", 0755)
		assert.Error(t, err)
	})
}

func TestIntegration_FsService_CreateFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("creates file via atomic upload", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser")
		require.NoError(t, err)

		// Initiate upload
		session, err := suite.ObjectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        13,
		})
		require.NoError(t, err)

		// Upload content
		data := []byte("test content")
		err = uploadToPresignedURL(session.UploadURL, data)
		require.NoError(t, err)

		// Complete upload via AtomicUpload
		file, err := suite.FsSvc.AtomicUpload(ctx, &vfs.AtomicUploadCommand{
			ObjectID: session.ObjectID,
			SystemID: sys.ID,
			DirPath:  "/home",
			FileName: "uploaded.txt",
			Mode:     inode.ModeObject | 0644,
		})
		require.NoError(t, err)
		assert.Equal(t, inode.ModeObject|0644, file.Mode())
		assert.Equal(t, int64(len(data)), file.Size())

		// Verify file can be resolved
		resolved, err := suite.FsSvc.ResolvePath(ctx, sys.ID, "/home/uploaded.txt")
		require.NoError(t, err)
		assert.Equal(t, file.ID(), resolved.ID())
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser2")
		require.NoError(t, err)

		// First upload
		session1, err := suite.ObjectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        13,
		})
		require.NoError(t, err)

		data1 := []byte("test content")
		err = uploadToPresignedURL(session1.UploadURL, data1)
		require.NoError(t, err)

		file1, err := suite.FsSvc.AtomicUpload(ctx, &vfs.AtomicUploadCommand{
			ObjectID: session1.ObjectID,
			SystemID: sys.ID,
			DirPath:  "/home",
			FileName: "overwrite.txt",
			Mode:     inode.ModeObject | 0644,
		})
		require.NoError(t, err)

		// Second upload (same path)
		session2, err := suite.ObjectSvc.InitiateUpload(ctx, &object.InitiateUploadCommand{
			SystemID:    sys.ID,
			ContentType: "text/plain",
			Size:        14,
		})
		require.NoError(t, err)

		data2 := []byte("updated content")
		err = uploadToPresignedURL(session2.UploadURL, data2)
		require.NoError(t, err)

		file2, err := suite.FsSvc.AtomicUpload(ctx, &vfs.AtomicUploadCommand{
			ObjectID: session2.ObjectID,
			SystemID: sys.ID,
			DirPath:  "/home",
			FileName: "overwrite.txt",
			Mode:     inode.ModeObject | 0644,
		})
		require.NoError(t, err)

		// Verify the new file has different ID (replaced)
		assert.NotEqual(t, file1.ID(), file2.ID())

		// Verify path resolves to new file
		resolved, err := suite.FsSvc.ResolvePath(ctx, sys.ID, "/home/overwrite.txt")
		require.NoError(t, err)
		assert.Equal(t, file2.ID(), resolved.ID())
	})
}

func TestIntegration_FsService_ListDirectory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("lists directory contents by system", func(t *testing.T) {
		_, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser")
		require.NoError(t, err)

		// Create directories
		_, err = suite.FsSvc.Mkdir(ctx, sys.ID, "/home/dir1", 0755)
		require.NoError(t, err)
		_, err = suite.FsSvc.Mkdir(ctx, sys.ID, "/home/dir2", 0755)
		require.NoError(t, err)

		// List by system ID
		entries, err := suite.FsSvc.List(ctx, &vfs.ListFilter{SystemID: &sys.ID})
		require.NoError(t, err)

		// Should contain multiple inodes (at least root, home, dir1, dir2)
		assert.GreaterOrEqual(t, len(entries), 4, "Should have at least 4 inodes")
	})
}

func TestIntegration_FsService_Delete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("deletes empty directory", func(t *testing.T) {
		u, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser")
		require.NoError(t, err)

		authCtx := suite.AuthenticatedContext(ctx, u.ID)

		_, err = suite.FsSvc.Mkdir(authCtx, sys.ID, "/home/todelete", 0755)
		require.NoError(t, err)

		err = suite.FsSvc.UnlinkPath(authCtx, sys.ID, "/home/todelete")
		require.NoError(t, err)

		// Verify directory no longer exists
		_, err = suite.FsSvc.ResolvePath(authCtx, sys.ID, "/home/todelete")
		assert.Error(t, err)
	})

	t.Run("rejects deleting non-empty directory", func(t *testing.T) {
		u, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser2")
		require.NoError(t, err)

		authCtx := suite.AuthenticatedContext(ctx, u.ID)

		_, err = suite.FsSvc.Mkdir(authCtx, sys.ID, "/home/nonempty", 0755)
		require.NoError(t, err)
		_, err = suite.FsSvc.Mkdir(authCtx, sys.ID, "/home/nonempty/child", 0755)
		require.NoError(t, err)

		err = suite.FsSvc.UnlinkPath(authCtx, sys.ID, "/home/nonempty")
		assert.Error(t, err)
	})
}

func TestIntegration_FsService_Rename(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start integration suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	t.Run("renames directory", func(t *testing.T) {
		u, sys, _, err := suite.SetupFullEnvironment(ctx, "testuser")
		require.NoError(t, err)

		authCtx := suite.AuthenticatedContext(ctx, u.ID)

		_, err = suite.FsSvc.Mkdir(authCtx, sys.ID, "/home/oldname", 0755)
		require.NoError(t, err)

		_, err = suite.FsSvc.Rename(authCtx, &vfs.RenameCommand{
			SystemID: sys.ID,
			Path:     "/home/oldname",
			NewName:  "newname",
		})
		require.NoError(t, err)

		// Verify old path no longer exists
		_, err = suite.FsSvc.ResolvePath(authCtx, sys.ID, "/home/oldname")
		assert.Error(t, err)

		// Verify new path exists
		_, err = suite.FsSvc.ResolvePath(authCtx, sys.ID, "/home/newname")
		require.NoError(t, err)
	})
}
