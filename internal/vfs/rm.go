package vfs

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Rm removes files or directories at the given paths. Partial-failure
// semantics are "stop on first error" (POSIX rm -f). Tombstones for
// deleted S3 objects are enqueued in the same transaction as the
// node unlinks, so a tombstone failure rolls back the rm.
//
// Permission is the caller's responsibility.
func (s *service) Rm(ctx context.Context, driveID string, paths []string, recursive bool) error {
	return s.tm.WithTx(ctx, func(ctx context.Context) error {
		rootID, err := s.GetRootNodeID(ctx, driveID)
		if err != nil {
			return err
		}
		var allRefs []GarbageRef
		for _, p := range paths {
			refs, err := s.rmPath(ctx, rootID, p, recursive)
			if err != nil {
				return err
			}
			allRefs = append(allRefs, refs...)
		}
		if len(allRefs) == 0 || s.GarbageRecorder == nil {
			return nil
		}
		if err := s.GarbageRecorder.RecordGarbage(ctx, allRefs); err != nil {
			return errorx.Wrap(err, fmt.Sprintf("vfs: rm tombstone enqueue (drive_id=%s, ref_count=%d)", driveID, len(allRefs)))
		}
		return nil
	})
}

func (s *service) rmPath(ctx context.Context, rootID uuid.UUID, path string, recursive bool) ([]GarbageRef, error) {
	out, err := newResolver(s.NodeClient).resolvePath(ctx, rootID, path, true)
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: rm resolve (path=%s)", path))
	}
	if out.Node == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: rm target not found (path="+path+")")
	}
	n := out.Node
	if n.IsDir() {
		if !recursive {
			return nil, errorx.New(errorx.KindFailedPrecondition, "vfs: rm target is a directory (path="+path+", use -r)")
		}
		return s.rmRecursive(ctx, rootID, n, path)
	}
	return s.rm(ctx, rootID, n, path)
}

// rm unlinks a single file node. When the last hardlink is removed
// the child is deleted; if it was an object node, the S3 reference
// is returned for tombstoning.
func (s *service) rm(ctx context.Context, rootID uuid.UUID, n *node.Node, path string) ([]GarbageRef, error) {
	if n == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: rm target not found (path="+path+")")
	}
	r := newResolver(s.NodeClient)
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
	if target := deleted; target != nil {
		n = target
	}
	if !n.IsObject() {
		return nil, nil
	}
	oc, err := n.ReadObject()
	if err != nil || oc.Bucket == "" || oc.Key == "" {
		return nil, nil
	}
	return []GarbageRef{{Bucket: oc.Bucket, Key: oc.Key}}, nil
}

func (s *service) rmRecursive(ctx context.Context, rootID uuid.UUID, n *node.Node, path string) ([]GarbageRef, error) {
	if n == nil {
		return nil, errorx.New(errorx.KindNotFound, "vfs: rm target not found (path="+path+")")
	}
	dc, err := n.ReadDir()
	if err != nil {
		return nil, errorx.Wrap(err, fmt.Sprintf("vfs: rm read dir (path=%s)", path))
	}
	var allRefs []GarbageRef
	for _, e := range dc.Entries {
		child, err := s.NodeClient.GetByID(ctx, e.InodeID)
		if err != nil {
			return nil, errorx.Wrap(err, fmt.Sprintf("vfs: rm get child (path=%s, child_id=%s)", path, e.InodeID))
		}
		if child == nil {
			return nil, errorx.New(errorx.KindNotFound, "vfs: rm child not found (path="+path+", child_id="+e.InodeID.String()+")")
		}
		childPath := strings.TrimRight(path, "/") + "/" + e.Name
		if child.IsDir() {
			refs, err := s.rmRecursive(ctx, rootID, child, childPath)
			if err != nil {
				return nil, err
			}
			allRefs = append(allRefs, refs...)
			continue
		}
		refs, err := s.rm(ctx, rootID, child, childPath)
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
