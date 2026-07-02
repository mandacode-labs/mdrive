package vfs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

// Rm removes files or directories at the given paths (like `rm [-r] path1 path2 ...`).
// Each path is removed in its own atomic node operation. Partial-failure
// semantics are "stop on first error": earlier successful removals stay
// committed, later ones are skipped. This matches POSIX rm -f.
//
// Tombstone records for the deleted S3 objects are enqueued after the
// node operations commit. If the post-commit enqueue fails, the rm is
// still reported as successful: the node state is already gone and the
// user-visible operation succeeded. Orphaned S3 objects can be reclaimed
// by a future orphan-scan job.
//
// Permission is the caller's responsibility.
func (s *Service) Rm(ctx context.Context, driveID string, paths []string, recursive bool) error {
	rootID, err := s.GetRootNodeID(ctx, driveID)
	if err != nil {
		return err
	}

	start := time.Now()
	var allRefs []GarbageRef
	for _, p := range paths {
		refs, err := s.rmPath(ctx, rootID, p, recursive)
		if err != nil {
			logx.Debug(ctx, "vfs.rm.failed",
				slog.String("drive_id", driveID),
				slog.String("path", p),
				slog.String("err", err.Error()),
			)
			return err
		}
		allRefs = append(allRefs, refs...)
	}

	if len(allRefs) > 0 && s.GarbageRecorder != nil {
		if err := s.GarbageRecorder.RecordGarbage(ctx, allRefs); err != nil {
			logx.Error(ctx, errorx.Wrap(err, fmt.Sprintf("vfs: rm post-commit tombstone enqueue (drive_id=%s, ref_count=%d)", driveID, len(allRefs))),
				"vfs.rm.tombstone_failed",
				slog.String("drive_id", driveID),
				slog.Int("ref_count", len(allRefs)),
			)
			return errorx.Wrap(err, fmt.Sprintf("vfs: rm post-commit tombstone enqueue (drive_id=%s, ref_count=%d)", driveID, len(allRefs)))
		}
		logx.Info(ctx, "vfs.rm.tombstoned",
			slog.String("drive_id", driveID),
			slog.Int("ref_count", len(allRefs)),
		)
	}

	logx.Debug(ctx, "vfs.rm.completed",
		slog.String("drive_id", driveID),
		slog.Int("path_count", len(paths)),
		slog.Bool("recursive", recursive),
		slog.Int("tombstoned", len(allRefs)),
		slog.Duration("elapsed", time.Since(start)),
	)
	return nil
}

// rmPath resolves the path and dispatches to the appropriate internal handler.
func (s *Service) rmPath(ctx context.Context, rootID uuid.UUID, path string, recursive bool) ([]GarbageRef, error) {
	r := s.newResolver()
	out, err := r.resolve(ctx, rootID, path, true)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: rm resolve (path=%s)", path))
	}
	n := out.Node
	if n.IsDir() {
		if !recursive {
			return nil, errorx.New(errorx.KindBadRequest, "vfs: rm target is a directory (path="+path+", use -r)")
		}
		return s.rmRecursive(ctx, rootID, n, path)
	}
	return s.rm(ctx, rootID, n, path, r)
}

// rm removes a single file node. Returns S3 references that need cleanup.
// The unlink delegates nlink management to node.Service; when the last
// hardlink is removed the child is deleted and any object body is
// returned as GarbageRef for tombstone registration.
func (s *Service) rm(ctx context.Context, rootID uuid.UUID, n *node.Node, path string, r *resolver) ([]GarbageRef, error) {
	parent, name, err := r.resolveParent(ctx, rootID, path)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: rm resolve parent (path=%s)", path))
	}
	if parent == nil || name == "" {
		return nil, nil
	}
	deleted, err := s.NodeClient.Unlink(ctx, parent, name)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: rm unlink (path=%s, name=%s)", path, name))
	}

	var refs []GarbageRef
	target := n
	if deleted != nil {
		target = deleted
	}
	if target.IsObject() {
		oc, err := target.ReadObject()
		switch {
		case err != nil:
			logx.Warn(ctx, "vfs.rm.read_object_content_failed",
				slog.String("err", err.Error()),
			)
		case oc.Bucket == "" || oc.Key == "":
			logx.Warn(ctx, "vfs.rm.object_content_empty",
				slog.String("err", "bucket or key empty"),
			)
		default:
			refs = append(refs, GarbageRef{Bucket: oc.Bucket, Key: oc.Key})
		}
	}
	return refs, nil
}

// rmRecursive removes a directory and all its children. Returns S3 references
// from all object nodes discovered during the traversal.
func (s *Service) rmRecursive(ctx context.Context, rootID uuid.UUID, n *node.Node, path string) ([]GarbageRef, error) {
	r := s.newResolver()
	dc, err := n.ReadDir()
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: rm read dir (path=%s)", path))
	}

	var allRefs []GarbageRef
	for _, e := range dc.Entries {
		childPath := strings.TrimRight(path, "/") + "/" + e.Name
		var child *node.Node
		if de, err := n.Lookup(e.Name); err == nil && de != nil {
			child, _ = r.loadByID(ctx, de.InodeID)
		}
		if child == nil {
			child, err = s.NodeClient.GetByID(ctx, e.InodeID)
			if err != nil {
				return nil, errorx.Wrap(err, fmt.Sprintf("vfs: rm get child (path=%s, child_id=%s)", childPath, e.InodeID))
			}
		}
		var refs []GarbageRef
		if child.IsDir() {
			refs, err = s.rmRecursive(ctx, rootID, child, childPath)
		} else {
			refs, err = s.rm(ctx, rootID, child, childPath, r)
		}
		if err != nil {
			return nil, err
		}
		allRefs = append(allRefs, refs...)
	}

	refs, err := s.rm(ctx, rootID, n, path, r)
	if err != nil {
		return nil, err
	}
	return append(allRefs, refs...), nil
}
