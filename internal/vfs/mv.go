package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/permission"
)

// Mv moves sources to dest (like `mv src1 src2 ... dest/`).
// Same-drive only. Executes within a node transaction so partial moves
// are automatically rolled back. If the destination entry exists as a
// file, it is overwritten (POSIX behavior).
//
// Semantics:
//   - Single source + non-existent destination path: rename.
//   - Single source + existing-directory destination: move into it.
//   - Multiple sources: destination must be an existing directory;
//     each source's basename is used.
func (s *Service) Mv(ctx context.Context, userID, srcDriveID string, srcPaths []string, dstDriveID, dstPath string) error {
	if srcDriveID != dstDriveID {
		return ErrCrossDrive
	}
	if err := s.checkAccess(ctx, userID, permission.PermissionEdit, srcDriveID); err != nil {
		return err
	}

	var overwriteRefs []ObjectRef
	if err := s.WithNodeTx(ctx, func(tx *Service) error {
		var err error
		if len(srcPaths) == 1 {
			overwriteRefs, err = tx.mvOne(ctx, srcDriveID, srcPaths[0], dstPath)
			return err
		}
		overwriteRefs, err = tx.mvBatch(ctx, srcDriveID, srcPaths, dstPath)
		return err
	}); err != nil {
		return err
	}

	if len(overwriteRefs) > 0 && s.GC != nil {
		if err := s.GC.InsertTombstones(ctx, overwriteRefs); err != nil {
			return fmt.Errorf("mv: post-commit tombstone enqueue failed (nodes already moved): %w", err)
		}
	}
	return nil
}

// mvOne moves a single source to dstPath. If the destination's last
// path component already exists as a file, it is overwritten; if it
// exists as a directory, the source is moved into it. The source path
// may traverse mounts but the resolved source must live in driveID
// (cross-drive moves are still rejected as ErrCrossDrive).
//
// Returns ObjectRef slices for any S3 objects that should be
// tombstoned (target overwritten, nlink hit zero). The caller is
// responsible for enqueueing them after the node transaction commits.
func (s *Service) mvOne(ctx context.Context, driveID, srcPath, dstPath string) ([]ObjectRef, error) {
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	srcRes, err := s.Resolve(ctx, driveID, srcPath)
	if err != nil {
		return nil, fmt.Errorf("mv: src %s: %w", srcPath, err)
	}
	if srcRes.DriveID != driveID {
		return nil, ErrCrossDrive
	}
	src := srcRes.Node
	dstParent, dstName, err := s.path.resolveParent(ctx, rootID, dstPath)
	if err != nil {
		return nil, fmt.Errorf("mv: dest: %w", err)
	}
	if !dstParent.IsDir() {
		return nil, fmt.Errorf("mv: dest: %w", ErrNotDirectory)
	}
	_, overwriteRefs, err := s.overwriteTarget(ctx, dstParent, dstName)
	if err != nil {
		return nil, fmt.Errorf("mv: overwrite target: %w", err)
	}
	srcParent, srcName, err := s.path.resolveParent(ctx, rootID, srcPath)
	if err != nil {
		return nil, fmt.Errorf("mv: resolve src parent: %w", err)
	}
	if srcParent != nil && srcName != "" {
		if _, err := s.Node.Unlink(ctx, srcParent, srcName); err != nil {
			return nil, fmt.Errorf("mv: unlink: %w", err)
		}
	}
	if err := s.Node.Link(ctx, dstParent, dstName, src); err != nil {
		return nil, fmt.Errorf("mv: link: %w", err)
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
func (s *Service) mvBatch(ctx context.Context, driveID string, srcPaths []string, dstPath string) ([]ObjectRef, error) {
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	dstOut, err := s.path.resolve(ctx, rootID, dstPath)
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
		res, err := s.Resolve(ctx, driveID, srcPath)
		if err != nil {
			return nil, fmt.Errorf("mv: %s: %w", srcPath, err)
		}
		if res.DriveID != driveID {
			return nil, ErrCrossDrive
		}
		sp, sn, err := s.path.resolveParent(ctx, rootID, srcPath)
		if err != nil {
			return nil, fmt.Errorf("mv: %s: resolve parent: %w", srcPath, err)
		}
		base := sn
		if base == "" {
			base = srcPath
		}
		if _, dup := seen[base]; dup {
			return nil, fmt.Errorf("mv: duplicate source basename %q in batch", base)
		}
		seen[base] = struct{}{}
		sources = append(sources, srcInfo{node: res.Node, baseName: base, srcParent: sp, srcName: sn})
	}

	for _, si := range sources {
		if si.node.ID() == dstDir.ID() {
			return nil, fmt.Errorf("mv: cannot move directory into itself")
		}
	}

	links := make(map[string]*node.Node, len(sources))
	var overwriteRefs []ObjectRef
	for _, si := range sources {
		_, refs, err := s.overwriteTarget(ctx, dstDir, si.baseName)
		if err != nil {
			return nil, fmt.Errorf("mv: dst overwrite: %w", err)
		}
		overwriteRefs = append(overwriteRefs, refs...)
		links[si.baseName] = si.node
	}

	byParent := make(map[*node.Node][]string)
	for _, si := range sources {
		if si.srcParent != nil && si.srcName != "" {
			byParent[si.srcParent] = append(byParent[si.srcParent], si.srcName)
		}
	}
	for parent, names := range byParent {
		if _, err := s.Node.BulkUnlink(ctx, parent, names); err != nil {
			return nil, fmt.Errorf("mv: bulk unlink: %w", err)
		}
	}

	if err := s.Node.BulkLink(ctx, dstDir, links); err != nil {
		return nil, fmt.Errorf("mv: bulk link: %w", err)
	}
	return overwriteRefs, nil
}

// overwriteTarget removes the existing entry at parent/name if it
// exists. Directories are rejected (POSIX: cannot overwrite a directory
// with a non-directory). Delegates nlink management to
// node.Service.UnlinkOrReplace so hardlinks are decremented and only
// deleted when nlink==0.
//
// Returns S3 references for any object node that was deleted
// (nlink reached 0); the caller is responsible for enqueueing
// tombstones. Returns nil refs when no entry was present or the
// target was a directory that was rejected.
func (s *Service) overwriteTarget(ctx context.Context, parent *node.Node, name string) (*node.Node, []ObjectRef, error) {
	entry, err := parent.Lookup(name)
	if err != nil {
		return nil, nil, nil
	}
	if entry == nil {
		return nil, nil, nil
	}
	existing, err := s.Node.GetByID(ctx, entry.InodeID)
	if err != nil {
		return nil, nil, fmt.Errorf("overwrite: get existing: %w", err)
	}
	if existing.IsDir() {
		return nil, nil, fmt.Errorf("cannot overwrite directory %q with a non-directory", name)
	}
	deleted, err := s.Node.UnlinkOrReplace(ctx, parent, name)
	if err != nil {
		return nil, nil, fmt.Errorf("overwrite: unlink: %w", err)
	}
	if deleted == nil {
		return existing, nil, nil
	}
	if deleted.IsObject() {
		oc, err := deleted.ReadObject()
		if err == nil && oc.Bucket != "" && oc.Key != "" {
			return existing, []ObjectRef{{Bucket: oc.Bucket, Key: oc.Key}}, nil
		}
	}
	return existing, nil, nil
}
