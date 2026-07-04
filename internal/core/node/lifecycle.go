package node

import (
	"context"

	"github.com/google/uuid"
)

// Lifecycle is the surface of a node service the upload flow
// needs: create object node, link into parent, delete on
// failure, and look up an existing node by ID.
type Lifecycle interface {
	CreateObject(ctx context.Context, content ObjectContent, size int64) (*Node, error)
	Link(ctx context.Context, parent *Node, name string, child *Node) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*Node, error)
}

var _ Lifecycle = (*Service)(nil)
