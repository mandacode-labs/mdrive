package node

import (
	"context"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/ent/node"
	entnode "github.com/mandacode-labs/mdrive/ent/node"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Repository is the data-access contract for nodes.
// Implementations live in the same package (entRepository) so they can access
// the Node's private mutators.
type SuperOperation interface {
	Read(ctx context.Context, id uuid.UUID) (*Node, error)
	Write(ctx context.Context, n *Node) error
	Destroy(ctx context.Context, id uuid.UUID) error
}

type superOperation struct {
	client *ent.Client
}

// Destroy implements [SuperOperation].
func (s *superOperation) Destroy(ctx context.Context, id uuid.UUID) error {
	client := s.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	err := client.Node.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errorx.New(errorx.KindNotFound, "node: not found")
		}
		return err
	}
	return nil
}

// Read implements [SuperOperation].
func (s *superOperation) Read(ctx context.Context, id uuid.UUID) (*Node, error) {
	client := s.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	e, err := client.Node.Query().Where(entnode.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errorx.New(errorx.KindNotFound, "node: not found")
		}
		return nil, err
	}
	return fromEnt(e), nil
}

// Write implements [SuperOperation].
func (s *superOperation) Write(ctx context.Context, n *Node) error {
	client := s.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	err := client.Node.Create().
		SetID(n.id).
		SetKind(entKind(n.kind)).
		SetSize(n.size).
		SetNlink(n.nlink).
		SetAtime(n.atime).
		SetMtime(n.mtime).
		SetCtime(n.ctime).
		SetCrtime(n.crtime).
		SetFlags(uint32(n.flags)).
		SetRevision(string(n.rev)).
		SetContent(n.content).
		OnConflictColumns(node.FieldID).
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return err
	}
	n.staleRev = n.rev
	return nil
}

func NewSuperOperation(client *ent.Client) SuperOperation {
	return &superOperation{client: client}
}

func fromEnt(e *ent.Node) *Node {
	if e == nil {
		return nil
	}
	rev := Revision(e.Revision)
	n := &Node{
		id:       e.ID,
		kind:     parseNodeKind(string(e.Kind)),
		size:     e.Size,
		nlink:    e.Nlink,
		atime:    e.Atime,
		mtime:    e.Mtime,
		ctime:    e.Ctime,
		crtime:   e.Crtime,
		flags:    Flags(e.Flags),
		rev:      rev,
		staleRev: rev,
	}
	if e.Content != nil {
		c := make(Content, len(*e.Content))
		copy(c, *e.Content)
		n.content = c
	}
	return n
}

func parseNodeKind(s string) NodeKind {
	switch s {
	case "file":
		return NodeKindFile
	case "directory":
		return NodeKindDirectory
	case "symlink":
		return NodeKindSymlink
	case "object":
		return NodeKindObject
	case "mount":
		return NodeKindMount
	default:
		return NodeKind(0)
	}
}

func entKind(k NodeKind) entnode.Kind {
	switch k {
	case NodeKindFile:
		return entnode.KindFile
	case NodeKindDirectory:
		return entnode.KindDirectory
	case NodeKindSymlink:
		return entnode.KindSymlink
	case NodeKindObject:
		return entnode.KindObject
	case NodeKindMount:
		return entnode.KindMount
	default:
		return entnode.KindFile
	}
}
