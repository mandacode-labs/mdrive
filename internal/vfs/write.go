package vfs

import (
	"context"
	"fmt"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// Write creates or overwrites inline content at path.
// Permission is the caller's responsibility.
func (s *Service) Write(ctx context.Context, driveID, path, content string) error {
	rootID, err := s.rootNodeID(ctx, driveID)
	if err != nil {
		return err
	}
	// A single resolver instance for the resolve + resolveParent
	// pair so any shared intermediate nodes see the same *Node.
	r := s.newResolver()
	out, err := r.resolve(ctx, rootID, path)
	if err != nil {
		// Path doesn't exist yet; treat the resolve error as a hint
		// to fall back to creating a new file. We need the parent
		// directory, so re-resolve with resolveParent.
		parent, name, perr := r.resolveParent(ctx, rootID, path)
		if perr != nil {
			return fmt.Errorf("write: %w", perr)
		}
		f, ferr := s.Node.CreateFile(ctx, content)
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
	return s.Node.Save(ctx, n)
}

// WriteLarge creates an object (S3-backed) node at path.
// Permission is the caller's responsibility.
func (s *Service) WriteLarge(ctx context.Context, driveID, path string, obj node.ObjectContent, size int64) error {
	parent, name, err := s.requireEditPath(ctx, driveID, path)
	if err != nil {
		return err
	}
	n, err := s.Node.CreateObject(ctx, obj, size)
	if err != nil {
		return err
	}
	return s.createAndLink(ctx, n, parent, name)
}
