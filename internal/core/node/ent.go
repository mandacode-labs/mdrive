package node

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/ent"
	entnode "github.com/mandacode-labs/mdrive/ent/node"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// entRepository is the Ent-backed implementation of Repository.
type entRepository struct {
	client *ent.Client
}

// NewRepository creates a new Ent-backed Repository.
func NewRepository(client *ent.Client) Repository {
	return &entRepository{client: client}
}

// Get returns the node with the given id, or ErrNotFound if it does not exist.
func (r *entRepository) Get(ctx context.Context, id uuid.UUID) (*Node, error) {
	e, err := r.client.Node.Query().Where(entnode.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errorx.New(errorx.KindNotFound, "node: not found")
		}
		return nil, err
	}
	return fromEnt(e), nil
}

// Save persists the node. On update, uses optimistic concurrency: the
// UPDATE is conditional on revision matching the value loaded from the DB.
func (r *entRepository) Save(ctx context.Context, n *Node) error {
	if n == nil {
		return errors.New("save: node is nil")
	}
	exists, err := r.existsTx(ctx, n.id)
	if err != nil {
		return err
	}
	content := n.content
	if !exists {
		if err := r.insert(ctx, n, content); err != nil {
			return err
		}
		n.staleRev = n.rev
		return nil
	}
	return r.update(ctx, n, content)
}

func (r *entRepository) existsTx(ctx context.Context, id uuid.UUID) (bool, error) {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	n, err := client.Node.Query().Where(entnode.IDEQ(id)).Exist(ctx)
	if err != nil {
		return false, err
	}
	return n, nil
}

func (r *entRepository) insert(ctx context.Context, n *Node, content Content) error {
	client := r.client
	tx, ok := entx.FromContext(ctx)
	if ok {
		client = tx.Client()
	}
	_, err := client.Node.Create().
		SetID(n.id).
		SetType(entType(n.kind)).
		SetSize(n.size).
		SetNlink(n.nlink).
		SetMode(n.mode).
		SetUID(n.uid).
		SetGid(n.gid).
		SetAtime(n.atime).
		SetMtime(n.mtime).
		SetCtime(n.ctime).
		SetCrtime(n.crtime).
		SetFlags(uint32(n.flags)).
		SetRevision(string(n.rev)).
		SetContent(content).
		Save(ctx)
	return err
}

func (r *entRepository) update(ctx context.Context, n *Node, content Content) error {
	client := r.client
	tx, ok := entx.FromContext(ctx)
	if ok {
		client = tx.Client()
	}
	affected, err := client.Node.Update().
		Where(entnode.IDEQ(n.id), entnode.RevisionEQ(string(n.staleRev))).
		SetType(entType(n.kind)).
		SetSize(n.size).
		SetNlink(n.nlink).
		SetMode(n.mode).
		SetUID(n.uid).
		SetGid(n.gid).
		SetAtime(n.atime).
		SetMtime(n.mtime).
		SetCtime(n.ctime).
		SetFlags(uint32(n.flags)).
		SetRevision(string(n.rev)).
		SetContent(content).
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return errorx.New(errorx.KindConflict, "node: revision conflict")
	}
	n.staleRev = n.rev
	return nil
}

// Delete removes the node with the given id.
func (r *entRepository) Delete(ctx context.Context, id uuid.UUID) error {
	client := r.client
	tx, ok := entx.FromContext(ctx)
	if ok {
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

func (r *entRepository) exists(ctx context.Context, id uuid.UUID) (bool, error) {
	n, err := r.client.Node.Query().Where(entnode.IDEQ(id)).Exist(ctx)
	if err != nil {
		return false, err
	}
	return n, nil
}

// fromEnt converts an ent.Node to a domain Node.
func fromEnt(e *ent.Node) *Node {
	if e == nil {
		return nil
	}
	rev := Revision(e.Revision)
	n := &Node{
		id:       e.ID,
		kind:     parseNodeType(string(e.Type)),
		size:     e.Size,
		nlink:    e.Nlink,
		mode:     e.Mode,
		uid:      e.UID,
		gid:      e.Gid,
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

func parseNodeType(s string) NodeType {
	switch s {
	case "file":
		return NodeTypeFile
	case "directory":
		return NodeTypeDirectory
	case "symlink":
		return NodeTypeSymlink
	case "object":
		return NodeTypeObject
	case "mount":
		return NodeTypeMount
	default:
		return NodeType(0)
	}
}

func entType(nt NodeType) entnode.Type {
	switch nt {
	case NodeTypeFile:
		return entnode.TypeFile
	case NodeTypeDirectory:
		return entnode.TypeDirectory
	case NodeTypeSymlink:
		return entnode.TypeSymlink
	case NodeTypeObject:
		return entnode.TypeObject
	case NodeTypeMount:
		return entnode.TypeMount
	default:
		return entnode.TypeFile
	}
}
