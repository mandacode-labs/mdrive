package handler

import (
	"context"

	coreuser "github.com/mandacode-labs/retrowin-go/internal/core/user"
)

// GroupService defines the subset of group operations needed by the handler layer.
type GroupService interface {
	Create(ctx context.Context, cmd *coreuser.GroupCreateCommand) (*coreuser.SystemGroup, error)
	Find(ctx context.Context, filter coreuser.GroupFilter) ([]*coreuser.SystemGroup, error)
	FindOne(ctx context.Context, filter coreuser.GroupFilter) (*coreuser.SystemGroup, error)
	Delete(ctx context.Context, id int) error
	AddUserToGroup(ctx context.Context, userSystemID, groupID int) error
	RemoveUserFromGroup(ctx context.Context, userSystemID, groupID int) error
}
