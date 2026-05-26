package fs

import (
	"context"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

func (s *service) Copy(ctx context.Context, id string, systemID string) (*inode.Inode, error) {
	original, err := s.inodeSvc.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.checkPermFromContext(ctx, original, AccessRead); err != nil {
		return nil, err
	}

	// Object inodes reference external storage - cannot simply copy
	if original.IsObject() {
		return nil, errors.BadRequest("cannot copy object inode: use upload service instead")
	}

	uid, err := s.userSvc.ResolveUID(ctx, systemID)
	if err != nil {
		return nil, err
	}

	return s.inodeSvc.Create(ctx, &inode.CreateCommand{
		SystemID: systemID,
		Mode:     original.Mode(),
		UID:      uid,
		GID:      original.GID(),
		Flags:    original.Flags(),
		Content:  original.Content(),
	})
}
