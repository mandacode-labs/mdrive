package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/internal/errors"
	"github.com/mandacode-labs/mdrive/internal/logging"
)

// Delete removes an inode by ID.
// Handles object cleanup and directory emptiness checks.
func (s *service) Delete(ctx context.Context, id string) error {
	in, err := s.inodeOps.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.checkPermFromContext(ctx, in, AccessWrite); err != nil {
		return err
	}

	switch {
	case in.IsObject():
		if err := s.deleteObjectRef(ctx, in); err != nil {
			return err
		}
	case in.IsDir():
		if !in.IsEmptyDir() {
			return errors.BadRequest("directory not empty")
		}
	}

	return s.inodeOps.Delete(ctx, id)
}

func (s *service) deleteObjectRef(ctx context.Context, in inodeGetter) error {
	objectID, err := in.ObjectID()
	if err != nil {
		return nil // Not an object or invalid content
	}
	if objectID == "" {
		return nil
	}
	if err := s.objectSvc.Delete(ctx, objectID); err != nil {
		if !errors.IsNotFound(err) && !ent.IsNotFound(err) {
			logging.Ctx(ctx).Warn().
				Str("object_id", objectID).
				Err(err).
				Msg("failed to delete object, skipping")
		}
	}
	return nil
}

type inodeGetter interface {
	ObjectID() (string, error)
}
