package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Mv moves sources to dst (like `mv src1 src2 ... dst/`). Same-drive
// only. Partial-failure for multi-source is "stop on first error"
// (POSIX mv). The whole batch runs inside one transaction, so
// either all sources move or none do; tombstone enqueue rolls back
// the move on failure.
//
// Permission is the caller's responsibility.
func (s *Service) Mv(ctx context.Context, srcDriveID string, srcPaths []string, dstDriveID, dstPath string) error {
	if srcDriveID != dstDriveID {
		return errorx.New(errorx.KindBadRequest, "vfs: cross-drive move not supported")
	}
	return s.tm.WithTx(ctx, func(ctx context.Context) error {
		var overwriteRefs []GarbageRef
		var err error
		if len(srcPaths) == 1 {
			overwriteRefs, err = s.mvOne(ctx, srcDriveID, srcPaths[0], dstPath)
		} else {
			overwriteRefs, err = s.mvBatch(ctx, srcDriveID, srcPaths, dstPath)
		}
		if err != nil {
			return err
		}
		if len(overwriteRefs) == 0 || s.GarbageRecorder == nil {
			return nil
		}
		if err := s.GarbageRecorder.RecordGarbage(ctx, overwriteRefs); err != nil {
			return errorx.Wrap(err, fmt.Sprintf("vfs: mv tombstone enqueue (drive_id=%s, ref_count=%d)", srcDriveID, len(overwriteRefs)))
		}
		return nil
	})
}

func (s *Service) mvOne(ctx context.Context, driveID, srcPath, dstPath string) ([]GarbageRef, error) {
	rootID, err := s.GetRootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	r := newResolver(s.NodeClient)
	srcOut, err := r.resolvePath(ctx, rootID, srcPath, true)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: mv resolve src (src_path=%s)", srcPath))
	}
	if srcOut.Remaining != "" {
		return nil, errorx.New(errorx.KindBadRequest, "vfs: cross-drive move not supported")
	}
	if srcOut.Node == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: mv src not found (src_path="+srcPath+")")
	}
	srcParent, srcName, err := r.resolveParent(ctx, rootID, srcPath)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: mv resolve src parent (src_path=%s)", srcPath))
	}
	if srcParent == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: mv src parent not found (src_path="+srcPath+")")
	}
	dstParent, dstName, err := r.resolveParent(ctx, rootID, dstPath)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: mv resolve dst parent (dst_path=%s)", dstPath))
	}
	if dstParent == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: mv dst parent not found (dst_path="+dstPath+")")
	}
	if !dstParent.IsDir() {
		return nil, errorx.Wrap(errorx.New(errorx.KindBadRequest, "vfs: not a directory"), fmt.Sprintf("vfs: mv dest (dst_path=%s)", dstPath))
	}
	return s.applyMoveEntry(ctx, srcParent, srcName, dstParent, dstName)
}

func (s *Service) mvBatch(ctx context.Context, driveID string, srcPaths []string, dstPath string) ([]GarbageRef, error) {
	rootID, err := s.GetRootNodeID(ctx, driveID)
	if err != nil {
		return nil, err
	}
	r := newResolver(s.NodeClient)
	dstOut, err := r.resolvePath(ctx, rootID, dstPath, true)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: mv resolve dst (dst_path=%s)", dstPath))
	}
	if dstOut.Node == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: mv dst not found (dst_path="+dstPath+")")
	}
	dstDir := dstOut.Node
	if !dstDir.IsDir() {
		return nil, errorx.Wrap(errorx.New(errorx.KindBadRequest, "vfs: not a directory"), fmt.Sprintf("vfs: mv dest (dst_path=%s)", dstPath))
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
		srcOut, err := r.resolvePath(ctx, rootID, srcPath, true)
		if err != nil {
			return nil, errorx.Wrap(err, fmt.Sprintf("vfs: mv batch resolve src (src_path=%s)", srcPath))
		}
		if srcOut.Remaining != "" {
			return nil, errorx.New(errorx.KindBadRequest, "vfs: cross-drive move not supported")
		}
		if srcOut.Node == nil {
			return nil, errorx.New(errorx.KindNotFound, "vfs: mv batch src not found (src_path="+srcPath+")")
		}
		sp, sn, err := r.resolveParent(ctx, rootID, srcPath)
		if err != nil {
			return nil, errorx.Wrap(err, fmt.Sprintf("vfs: mv batch resolve src parent (src_path=%s)", srcPath))
		}
		if sp == nil {
			return nil, errorx.New(errorx.KindNotFound, "vfs: mv batch src parent not found (src_path="+srcPath+")")
		}
		if _, dup := seen[sn]; dup {
			return nil, errorx.New(errorx.KindBadRequest, "vfs: mv duplicate source basename "+sn+" in batch")
		}
		seen[sn] = struct{}{}
		sources = append(sources, srcInfo{node: srcOut.Node, baseName: sn, srcParent: sp, srcName: sn})
	}

	for _, si := range sources {
		if si.node.ID() == dstDir.ID() {
			return nil, errorx.New(errorx.KindBadRequest, "vfs: mv cannot move directory into itself")
		}
	}

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

// applyMoveEntry captures the S3 reference of the overwrite target
// (if any) before MoveEntry removes it, then delegates to
// node.Service.MoveEntry. The child inode is preserved (no nlink
// bookkeeping here) — that's MoveEntry's responsibility.
func (s *Service) applyMoveEntry(ctx context.Context, srcParent *node.Node, srcName string, dstParent *node.Node, dstName string) ([]GarbageRef, error) {
	if srcParent == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: mv src parent not found (src_name="+srcName+")")
	}
	if dstParent == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: mv dst parent not found (dst_name="+dstName+")")
	}
	overwriteRef, err := s.captureOverwriteRef(ctx, dstParent, dstName)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: mv capture overwrite ref (dst_name=%s)", dstName))
	}
	if err := s.NodeClient.MoveEntry(ctx, srcParent, srcName, dstParent, dstName); err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: mv move entry (src_name=%s, dst_name=%s)", srcName, dstName))
	}
	if overwriteRef != nil {
		return []GarbageRef{*overwriteRef}, nil
	}
	return nil, nil
}

func (s *Service) captureOverwriteRef(ctx context.Context, dstParent *node.Node, dstName string) (*GarbageRef, error) {
	if dstParent == nil {
		return nil, nil
	}
	existing, err := dstParent.Lookup(dstName)
	if err != nil || existing == nil {
		return nil, nil
	}
	child, err := s.NodeClient.GetByID(ctx, existing.InodeID)
	if err != nil || child == nil || !child.IsObject() {
		return nil, nil
	}
	oc, err := child.ReadObject()
	if err != nil || oc.Bucket == "" || oc.Key == "" {
		return nil, nil
	}
	return &GarbageRef{Bucket: oc.Bucket, Key: oc.Key}, nil
}
