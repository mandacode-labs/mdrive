package node

import (
	"context"

	"github.com/google/uuid"
)

// Linker is the read+mutate surface of a node service that
// orchestrates the inode tree (linking, unlinking, moving,
// saving, lookup). vfs uses this to walk and rewrite the
// tree without ever creating a node itself.
type Linker interface {
	Link(ctx context.Context, parent *Node, name string, child *Node) error
	Unlink(ctx context.Context, parent *Node, name string) (*Node, error)
	MoveEntry(ctx context.Context, srcParent *Node, srcName string, dstParent *Node, dstName string) error
	GetByID(ctx context.Context, id uuid.UUID) (*Node, error)
	Save(ctx context.Context, n *Node) error
}

var _ Linker = (*Service)(nil)
