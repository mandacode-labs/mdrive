package node

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the data-access contract for nodes.
// Implementations live in the same package (entRepository) so they can access
// the Node's private mutators.
type Repository interface {
	// Get returns the node with the given id.
	Get(ctx context.Context, id uuid.UUID) (*Node, error)

	// Save persists the node (insert if new, update otherwise).
	Save(ctx context.Context, n *Node) error

	// Delete removes the node with the given id.
	Delete(ctx context.Context, id uuid.UUID) error
}
