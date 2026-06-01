package fs

import (
	"context"
	"path"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

// DeleteRecursive recursively deletes a directory and all its contents.
// Strategy: depth-first search, deleting leaf nodes first.
// For each object inode: S3 is deleted BEFORE the DB record to prevent orphan storage costs.
// Permission check is done only on the top-level directory (Unix rm -r behavior).
func (s *service) DeleteRecursive(ctx context.Context, systemID string, dirPath string) error {
	// Resolve the target directory
	targetInode, err := s.ResolvePath(ctx, systemID, dirPath)
	if err != nil {
		return err
	}

	if !targetInode.IsDir() {
		return errors.BadRequest("not a directory")
	}

	// Check write permission on the top-level directory only
	if err := s.checkPermFromContext(ctx, targetInode, AccessWrite); err != nil {
		return err
	}

	// Get parent directory for final unlink
	parentDirPath := path.Dir(dirPath)
	entryName := path.Base(dirPath)
	if parentDirPath == dirPath {
		return errors.BadRequest("cannot delete root directory")
	}

	// Recursively delete contents
	if err := s.deleteRecursiveContents(ctx, systemID, targetInode); err != nil {
		return err
	}

	// Unlink from parent directory (needs lock)
	parentDir, err := s.ResolvePath(ctx, systemID, parentDirPath)
	if err != nil {
		return err
	}

	unlock := s.dirLock.Lock(parentDir.ID())
	defer unlock()

	if err := s.dentrySvc.Unlink(ctx, parentDir.ID(), entryName); err != nil {
		return err
	}

	return s.inodeSvc.Delete(ctx, targetInode.ID())
}

// deleteRecursiveContents deletes all entries within a directory recursively.
// S3 objects are deleted before DB records.
func (s *service) deleteRecursiveContents(ctx context.Context, systemID string, dirInode *inode.Inode) error {
	entries, err := dirInode.ReadDir()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Name == "." || entry.Name == ".." {
			continue
		}

		childInode, err := s.inodeSvc.GetByID(ctx, entry.InodeID)
		if err != nil {
			return err
		}

		if childInode.IsDir() {
			// Recursively delete subdirectory
			if err := s.deleteRecursiveContents(ctx, systemID, childInode); err != nil {
				return err
			}
			// Delete the empty directory inode
			if err := s.inodeSvc.Delete(ctx, childInode.ID()); err != nil {
				return err
			}
		} else {
			// Delete object from S3 BEFORE DB to prevent orphan storage
			if childInode.IsObject() {
				if err := s.deleteObjectRef(ctx, childInode); err != nil {
					return err
				}
			}
			// Delete inode (and its object record if any)
			if err := s.inodeSvc.Delete(ctx, childInode.ID()); err != nil {
				return err
			}
		}

		// Remove entry from current directory (best-effort, will be cleaned by final parent unlink)
		_ = s.dentrySvc.Unlink(ctx, dirInode.ID(), entry.Name)
	}

	return nil
}
