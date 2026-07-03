package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Write creates or overwrites inline content at path.
// Permission is the caller's responsibility.
//
// The create-on-missing branch constructs the file in memory
// and hands it to Node.Link, which inserts the row inside the
// same transaction as the parent's directory update. The
// overwrite branch goes through Node.Save, whose UPDATE is
// committed atomically.
func (s *Service) Write(ctx context.Context, driveID, path, content string) error {
	rootID, err := s.GetRootNodeID(ctx, driveID)
	if err != nil {
		return err
	}
	r := newResolver(s.NodeClient)
	out, err := r.resolvePath(ctx, rootID, path, true)
	if err != nil {
		parent, name, perr := r.resolveParent(ctx, rootID, path)
		if perr != nil {
			return errorx.Wrap(perr, fmt.Sprintf("vfs: write resolve parent (path=%s)", path))
		}
		if parent == nil {
			return errorx.New(errorx.KindNotFound, "vfs: write parent not found (path="+path+")")
		}
		f, ferr := node.NewFile(content)
		if ferr != nil {
			return errorx.Wrap(ferr, fmt.Sprintf("vfs: write new file (path=%s)", path))
		}
		return s.NodeClient.Link(ctx, parent, name, f)
	}
	if out.Node == nil {
		return errorx.New(errorx.KindNotFound, "vfs: write target not found (path="+path+")")
	}
	n := out.Node
	if !n.IsFile() {
		return errorx.New(errorx.KindBadRequest, "vfs: write target is not a file (type="+string(n.Kind())+")")
	}
	if err := n.WriteFile(content); err != nil {
		return errorx.Wrap(err, fmt.Sprintf("vfs: write encode content (path=%s)", path))
	}
	if err := s.NodeClient.Save(ctx, n); err != nil {
		return errorx.Wrap(err, fmt.Sprintf("vfs: write save (path=%s)", path))
	}
	return nil
}

// WriteLarge creates an object (S3-backed) node at path.
// Permission is the caller's responsibility.
func (s *Service) WriteLarge(ctx context.Context, driveID, path string, obj node.ObjectContent, size int64) error {
	parent, name, err := s.resolveEditableParent(ctx, driveID, path)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("vfs: write_large resolve parent (drive_id=%s, path=%s)", driveID, path))
	}
	if parent == nil {
		return errorx.New(errorx.KindNotFound, "vfs: write_large parent not found (drive_id="+driveID+", path="+path+")")
	}
	n, err := node.NewObject(obj, size)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("vfs: write_large new object (drive_id=%s, path=%s)", driveID, path))
	}
	return s.NodeClient.Link(ctx, parent, name, n)
}
