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

// Lifecycle is the surface of a node service the upload flow
// needs: create object node, link into parent, delete on
// failure, and look up an existing node by ID.
type Lifecycle interface {
	CreateObject(ctx context.Context, content ObjectContent, size int64) (*Node, error)
	Link(ctx context.Context, parent *Node, name string, child *Node) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*Node, error)
}

var (
	_ Linker    = (*Service)(nil)
	_ Lifecycle = (*Service)(nil)
)
