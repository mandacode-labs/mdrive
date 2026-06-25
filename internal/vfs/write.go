package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
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
	r := s.newResolver()
	out, err := r.resolve(ctx, rootID, path, true)
	if err != nil {
		parent, name, perr := r.resolveParent(ctx, rootID, path)
		if perr != nil {
			return fmt.Errorf("write: %w", perr)
		}
		f, ferr := node.NewFile(content)
		if ferr != nil {
			return ferr
		}
		return s.createAndLink(ctx, f, parent, name)
	}
	n := out.Node
	if !n.IsFile() {
		return fmt.Errorf("write: cannot write to %s", n.Type())
	}
	if err := n.WriteFile(content); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return s.NodeClient.Save(ctx, n)
}

// WriteLarge creates an object (S3-backed) node at path.
// Permission is the caller's responsibility.
func (s *Service) WriteLarge(ctx context.Context, driveID, path string, obj node.ObjectContent, size int64) error {
	parent, name, err := s.requireEditPath(ctx, driveID, path)
	if err != nil {
		return err
	}
	n, err := node.NewObject(obj, size)
	if err != nil {
		return err
	}
	return s.createAndLink(ctx, n, parent, name)
}
