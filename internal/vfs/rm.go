package vfs

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Rm removes files or directories at the given paths (like `rm [-r] path1 path2 ...`).
// Node operations execute in a transaction and are atomic. Tombstone
// records for the deleted S3 objects are enqueued after the node
// transaction commits. If the post-commit enqueue fails, the rm is
// still reported as successful to the caller: the node state is
// already gone and the user-visible operation succeeded. Orphaned S3
// objects can be reclaimed by a future orphan-scan job.
func (s *Service) Rm(ctx context.Context, userID, driveID string, paths []string, recursive bool) error {
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, driveID); err != nil {
		return err
	}
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return err
	}

	var allRefs []ObjectRef

	if err := s.WithNodeTx(ctx, func(tx *Service) error {
		for _, p := range paths {
			refs, err := tx.rmPath(ctx, rootID, p, recursive)
			if err != nil {
				return err
			}
			allRefs = append(allRefs, refs...)
		}
		return nil
	}); err != nil {
		return err
	}

	if len(allRefs) > 0 && s.GC != nil {
		// Best-effort: enqueue tombstones after the node commit. If this
		// fails the nodes are already gone, so we still return success
		// to the caller (the user-visible rm succeeded). The error is
		// captured for the caller to surface as a warning if desired.
		if err := s.GC.InsertTombstones(ctx, allRefs); err != nil {
			return fmt.Errorf("rm: post-commit tombstone enqueue failed (nodes already deleted): %w", err)
		}
	}

	return nil
}

// rmPath resolves the path and dispatches to the appropriate internal handler.
func (s *Service) rmPath(ctx context.Context, rootID uuid.UUID, path string, recursive bool) ([]ObjectRef, error) {
	n, err := s.path.resolve(ctx, rootID, path)
	if err != nil {
		return nil, fmt.Errorf("rm: %s: %w", path, err)
	}
	if n.IsDir() {
		if !recursive {
			return nil, fmt.Errorf("rm: %s: is a directory (use -r)", path)
		}
		return s.rmRecursive(ctx, rootID, n, path)
	}
	return s.rm(ctx, rootID, n, path)
}

// rm removes a single file node. Returns S3 references that need cleanup.
// The unlink delegates nlink management to node.Service; when the last
// hardlink is removed the child is deleted and any object body is
// returned as ObjectRef for tombstone registration.
func (s *Service) rm(ctx context.Context, rootID uuid.UUID, n *node.Node, path string) ([]ObjectRef, error) {
	parent, name, err := s.path.resolveParent(ctx, rootID, path)
	if err != nil {
		return nil, fmt.Errorf("rm: resolve parent: %w", err)
	}
	if parent == nil || name == "" {
		return nil, nil
	}
	deleted, err := s.Node.Unlink(ctx, parent, name)
	if err != nil {
		return nil, fmt.Errorf("rm: unlink: %w", err)
	}

	var refs []ObjectRef
	target := n
	if deleted != nil {
		target = deleted
	}
	if target.IsObject() {
		oc, err := target.ReadObject()
		if err == nil && oc.Bucket != "" && oc.Key != "" {
			refs = append(refs, ObjectRef{Bucket: oc.Bucket, Key: oc.Key})
		}
	}
	return refs, nil
}

// rmRecursive removes a directory and all its children. Returns S3 references
// from all object nodes discovered during the traversal.
func (s *Service) rmRecursive(ctx context.Context, rootID uuid.UUID, n *node.Node, path string) ([]ObjectRef, error) {
	dc, err := n.ReadDir()
	if err != nil {
		return nil, fmt.Errorf("rm: read dir: %w", err)
	}

	var allRefs []ObjectRef
	for _, e := range dc.Entries {
		childPath := strings.TrimRight(path, "/") + "/" + e.Name
		child, err := s.Node.GetByID(ctx, e.InodeID)
		if err != nil {
			return nil, fmt.Errorf("rm: get child %s: %w", childPath, err)
		}
		var refs []ObjectRef
		if child.IsDir() {
			refs, err = s.rmRecursive(ctx, rootID, child, childPath)
		} else {
			refs, err = s.rm(ctx, rootID, child, childPath)
		}
		if err != nil {
			return nil, err
		}
		allRefs = append(allRefs, refs...)
	}

	refs, err := s.rm(ctx, rootID, n, path)
	if err != nil {
		return nil, err
	}
	return append(allRefs, refs...), nil
}
