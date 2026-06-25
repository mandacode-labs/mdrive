package vfs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Mv moves sources to dest (like `mv src1 src2 ... dest/`).
// Same-drive only. The underlying node.Service.MoveEntry is atomic
// on its own (it wraps its writes in WithTx), so vfs just orchestrates
// the resolve + move and reports S3 references for any overwritten
// object that should be tombstoned after commit.
//
// Semantics:
//   - Single source + non-existent destination path: rename.
//   - Single source + existing-directory destination: move into it.
//   - Multiple sources: destination must be an existing directory;
//     each source's basename is used.
//
// Permission is the caller's responsibility.
func (s *Service) Mv(ctx context.Context, srcDriveID string, srcPaths []string, dstDriveID, dstPath string) error {
	if srcDriveID != dstDriveID {
		return ErrCrossDrive
	}

	start := time.Now()
	var (
		overwriteRefs []GarbageRef
		err           error
	)
	if len(srcPaths) == 1 {
		overwriteRefs, err = s.mvOne(ctx, srcDriveID, srcPaths[0], dstPath)
	} else {
		overwriteRefs, err = s.mvBatch(ctx, srcDriveID, srcPaths, dstPath)
	}
	if err != nil {
		s.log().Debug("vfs.mv.failed",
			slog.String("drive_id", srcDriveID),
			slog.Int("src_count", len(srcPaths)),
			slog.String("dst_path", dstPath),
			slog.String("err", err.Error()),
		)
		return err
	}

	if len(overwriteRefs) > 0 && s.GarbageRecorder != nil {
		if err := s.GarbageRecorder.RecordGarbage(ctx, overwriteRefs); err != nil {
			s.log().Error("vfs.mv.tombstone_failed",
				slog.String("drive_id", srcDriveID),
				slog.Int("ref_count", len(overwriteRefs)),
				slog.String("err", err.Error()),
			)
			return fmt.Errorf("mv: post-commit tombstone enqueue failed (nodes already moved): %w", err)
		}
		s.log().Info("vfs.mv.tombstoned",
			slog.String("drive_id", srcDriveID),
			slog.Int("ref_count", len(overwriteRefs)),
		)
	}

	s.log().Debug("vfs.mv.completed",
		slog.String("drive_id", srcDriveID),
		slog.Int("src_count", len(srcPaths)),
		slog.String("dst_path", dstPath),
		slog.Int("tombstoned", len(overwriteRefs)),
		slog.Duration("elapsed", time.Since(start)),
	)
	return nil
}

// mvOne moves a single source to dstPath. If the destination's last
// path component already exists as a file, it is overwritten; if it
// exists as a directory, the source is moved into it. The source path
// may traverse mounts but the resolved source must live in driveID
// (cross-drive moves are still rejected as ErrCrossDrive).
//
// Returns GarbageRef slices for any S3 objects that should be
// tombstoned (target overwritten, nlink hit zero). The caller is
// responsible for enqueueing them after the node transaction commits.
//
// The move is delegated to node.Service.MoveEntry, which preserves
// the child inode's nlink instead of doing the Unlink + Link pair.
// The Unlink + Link pair is unsafe for nlink==1 (it deletes the
// inode) and is also unsafe when combined with concurrent loads
// (the in-memory child becomes stale across the operation). The
// high-level MoveEntry sidesteps both issues.
//
// A single fresh resolver is used for the resolveParent pair so
// that when src and dst share a parent directory, the two calls
// return the same *Node pointer. MoveEntry is responsible for
// updating both parents atomically; the resolver cache keeps the
// optimistic-concurrency check in node.Repository.Save consistent.
func (s *Service) mvOne(ctx context.Context, driveID, srcPath, dstPath string) ([]GarbageRef, error) {
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	r := s.newResolver()
	// A single resolve call gives us the src node; if it stops at a
	// mount, the move would cross drives, which mv rejects.
	srcOut, err := r.resolve(ctx, rootID, srcPath, true)
	if err != nil {
		return nil, fmt.Errorf("mv: src %s: %w", srcPath, err)
	}
	if srcOut.Remaining != "" {
		return nil, ErrCrossDrive
	}
	srcParent, srcName, err := r.resolveParent(ctx, rootID, srcPath)
	if err != nil {
		return nil, fmt.Errorf("mv: resolve src parent: %w", err)
	}
	dstParent, dstName, err := r.resolveParent(ctx, rootID, dstPath)
	if err != nil {
		return nil, fmt.Errorf("mv: dest: %w", err)
	}
	if !dstParent.IsDir() {
		return nil, fmt.Errorf("mv: dest: %w", ErrNotDirectory)
	}

	overwriteRefs, err := s.applyMoveEntry(ctx, srcParent, srcName, dstParent, dstName)
	if err != nil {
		return nil, err
	}
	return overwriteRefs, nil
}

// mvBatch handles multi-source moves. The destination must resolve to
// an existing directory; sources are moved in keeping their basenames.
// All sources must resolve to driveID (cross-drive moves via mount
// traversal are rejected as ErrCrossDrive).
//
// Returns S3 references for any overwritten destination entry whose
// nlink hit zero; caller enqueues tombstones after the node tx commits.
//
// As in mvOne, each move is delegated to MoveEntry so the child
// inode's nlink is preserved. A single fresh resolver is shared
// across the resolveParent calls so multiple sources that share
// a parent directory see the same *Node pointer.
func (s *Service) mvBatch(ctx context.Context, driveID string, srcPaths []string, dstPath string) ([]GarbageRef, error) {
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	r := s.newResolver()
	dstOut, err := r.resolve(ctx, rootID, dstPath, true)
	if err != nil {
		return nil, fmt.Errorf("mv: dest: %w", err)
	}
	dstDir := dstOut.Node
	if !dstDir.IsDir() {
		return nil, fmt.Errorf("mv: dest: %w", ErrNotDirectory)
	}

	type srcInfo struct {
		node      *node.Node
		baseName  string
		srcParent *node.Node
		srcName   string
	}
	sources := make([]srcInfo, 0, len(srcPaths))
	seen := make(map[string]struct{}, len(srcPaths))
	for _, srcPath := range srcPaths {
		srcOut, err := r.resolve(ctx, rootID, srcPath, true)
		if err != nil {
			return nil, fmt.Errorf("mv: %s: %w", srcPath, err)
		}
		if srcOut.Remaining != "" {
			return nil, ErrCrossDrive
		}
		sp, sn, err := r.resolveParent(ctx, rootID, srcPath)
		if err != nil {
			return nil, fmt.Errorf("mv: %s: resolve parent: %w", srcPath, err)
		}
		if _, dup := seen[sn]; dup {
			return nil, fmt.Errorf("mv: duplicate source basename %q in batch", sn)
		}
		seen[sn] = struct{}{}
		sources = append(sources, srcInfo{node: srcOut.Node, baseName: sn, srcParent: sp, srcName: sn})
	}

	for _, si := range sources {
		if si.node.ID() == dstDir.ID() {
			return nil, fmt.Errorf("mv: cannot move directory into itself")
		}
	}

	// Each move is its own atomic operation. Partial-failure semantics
	// for a multi-source batch are "stop on first error": earlier
	// successful moves stay committed, later ones are skipped. This
	// matches POSIX mv: a partial batch is not rolled back even when
	// one source fails. Callers needing all-or-nothing semantics must
	// stage sources into a temp directory and rename that atomically.
	var overwriteRefs []GarbageRef
	for _, si := range sources {
		refs, err := s.applyMoveEntry(ctx, si.srcParent, si.srcName, dstDir, si.baseName)
		if err != nil {
			return nil, err
		}
		overwriteRefs = append(overwriteRefs, refs...)
	}
	return overwriteRefs, nil
}

// applyMoveEntry wraps node.Service.MoveEntry with vfs-level concerns:
// detecting when the overwrite target is an S3-backed object (whose
// bucket+key must be tombstoned for GC) and translating node errors
// into vfs errors.
func (s *Service) applyMoveEntry(ctx context.Context, srcParent *node.Node, srcName string, dstParent *node.Node, dstName string) ([]GarbageRef, error) {
	// Capture the overwrite target's S3 reference (if any) before
	// MoveEntry removes it. After the call the inode may be gone
	// (nlink hit 0), so we need the reference pre-emptively.
	var overwriteRef *GarbageRef
	if existing, err := dstParent.Lookup(dstName); err == nil && existing != nil {
		if existingChild, err := s.NodeClient.GetByID(ctx, existing.InodeID); err == nil && existingChild.IsObject() {
			if oc, err := existingChild.ReadObject(); err == nil && oc.Bucket != "" && oc.Key != "" {
				overwriteRef = &GarbageRef{Bucket: oc.Bucket, Key: oc.Key}
			}
		}
	}

	if err := s.NodeClient.MoveEntry(ctx, srcParent, srcName, dstParent, dstName); err != nil {
		return nil, fmt.Errorf("mv: %w", err)
	}

	if overwriteRef != nil {
		return []GarbageRef{*overwriteRef}, nil
	}
	return nil, nil
}
