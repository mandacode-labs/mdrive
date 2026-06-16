package node

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/ent"
	entnode "github.com/mandacode-labs/mdrive/ent/node"
)

// entRepository is the Ent-backed implementation of Repository.
// It is intentionally unexported: external callers obtain a Repository via
// NewEntRepository, which returns the interface.
type entRepository struct {
	client *ent.Client
}

// NewEntRepository creates a new Ent-backed Repository.
func NewEntRepository(client *ent.Client) Repository {
	return &entRepository{client: client}
}

// Get returns the node with the given id, or ErrNotFound if it does not exist.
func (r *entRepository) Get(ctx context.Context, id uuid.UUID) (*Node, error) {
	e, err := r.client.Node.Query().Where(entnode.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return fromEnt(e), nil
}

// GetByID is an alias for Get, kept for clarity at call sites.
func (r *entRepository) GetByID(ctx context.Context, id uuid.UUID) (*Node, error) {
	return r.Get(ctx, id)
}

// Save persists the node. If the node already exists (by id), it is updated;
// otherwise it is inserted.
func (r *entRepository) Save(ctx context.Context, n *Node) error {
	if n == nil {
		return errors.New("save: node is nil")
	}
	exists, err := r.exists(ctx, n.id)
	if err != nil {
		return err
	}
	content := n.content
	if !exists {
		_, err := r.client.Node.Create().
			SetID(n.id).
			SetType(entnode.Type(n.typ)).
			SetStatus(entnode.Status(n.status)).
			SetSize(n.size).
			SetNlink(n.nlink).
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
	_, err = r.client.Node.UpdateOneID(n.id).
		SetType(entnode.Type(n.typ)).
		SetStatus(entnode.Status(n.status)).
		SetSize(n.size).
		SetNlink(n.nlink).
		SetAtime(n.atime).
		SetMtime(n.mtime).
		SetCtime(n.ctime).
		SetFlags(uint32(n.flags)).
		SetRevision(string(n.rev)).
		SetContent(content).
		Save(ctx)
	return err
}

// Delete removes the node with the given id.
func (r *entRepository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.client.Node.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// WithTx executes fn within a transaction. The Repository passed to fn
// operates on the transaction, so its operations are atomic.
func (r *entRepository) WithTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	txClient := tx.Client()
	txRepo := &entRepository{client: txClient}
	if err := fn(txRepo); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// exists checks whether a node with the given id exists.
func (r *entRepository) exists(ctx context.Context, id uuid.UUID) (bool, error) {
	n, err := r.client.Node.Query().Where(entnode.IDEQ(id)).Exist(ctx)
	if err != nil {
		return false, err
	}
	return n, nil
}

// fromEnt converts an ent.Node to a domain Node, populating all private fields.
func fromEnt(e *ent.Node) *Node {
	if e == nil {
		return nil
	}
	n := &Node{
		id:     e.ID,
		typ:    parseNodeType(string(e.Type)),
		status: parseNodeStatus(string(e.Status)),
		size:   e.Size,
		nlink:  e.Nlink,
		atime:  e.Atime,
		mtime:  e.Mtime,
		ctime:  e.Ctime,
		crtime: e.Crtime,
		flags:  Flags(e.Flags),
		rev:    Revision(e.Revision),
	}
	if e.Content != nil {
		// Copy the slice so that the domain Node owns its content.
		c := make(Content, len(*e.Content))
		copy(c, *e.Content)
		n.content = c
	}
	return n
}

// parseNodeType converts the ent string enum back into the domain NodeType.
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
	case "device":
		return NodeTypeDevice
	default:
		return NodeType(0) // unknown
	}
}

// parseNodeStatus converts the ent string enum back into the domain Status.
func parseNodeStatus(s string) Status {
	switch s {
	case "pending":
		return StatusPending
	case "active":
		return StatusActive
	case "pending_delete":
		return StatusPendingDelete
	case "missing":
		return StatusMissing
	default:
		return StatusActive
	}
}

// now is overridable in tests.
var now = time.Now
