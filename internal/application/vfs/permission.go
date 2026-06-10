package vfs

import (
	"context"

	"github.com/mandacode-labs/mdrive/internal/core/inode"
)

// AccessType is an alias for inode.AccessType for backward compatibility.
type AccessType = inode.AccessType

const (
	AccessRead    = inode.AccessRead
	AccessWrite   = inode.AccessWrite
	AccessExecute = inode.AccessExecute
)

// checkPermFromContext extracts user info from context and checks permissions.
func (s *service) checkPermFromContext(ctx context.Context, in *inode.Inode, access AccessType) error {
	uid, gids, err := s.userSvc.ResolveUIDAndGIDs(ctx, in.SystemID())
	if err != nil {
		return err
	}

	return in.CheckPerm(uid, gids, access)
}
