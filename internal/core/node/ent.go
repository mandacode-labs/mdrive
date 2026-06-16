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

// GetRoot returns the root directory node of the given drive.
// A drive has exactly one root node, identified by being the only node in the drive
// whose parent is implicit. We treat the first node found with the driveID as the root;
// the unique constraint is enforced by the application (Service.NewRootNode assigns
// the root flag implicitly via the absence of a parent link).
func (r *entRepository) GetRoot(ctx context.Context, driveID string) (*Node, error) {
	// Root node convention: the only node in the drive with no parent reference.
	// In the current schema, no node has a parent_id (Linux-style: parent is in
	// the parent directory's content). For now, the first node inserted for a drive
	// is treated as the root. A more explicit approach would add a drive_roots table
	// or a root flag, deferred to the drive package.
	e, err := r.client.Node.Query().
		Where(entnode.DriveIDEQ(driveID)).
		Order(entnode.ByCreateTime()).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return fromEnt(e), nil
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
	content := contentToBytes(n.content)
	if !exists {
		_, err := r.client.Node.Create().
			SetID(n.id).
			SetDriveID(n.driveID).
			SetType(entnode.Type(n.typ)).
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
		SetDriveID(n.driveID).
		SetType(entnode.Type(n.typ)).
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
		id:      e.ID,
		driveID: e.DriveID,
		typ:     parseNodeType(string(e.Type)),
		size:    e.Size,
		nlink:   e.Nlink,
		atime:   e.Atime,
		mtime:   e.Mtime,
		ctime:   e.Ctime,
		crtime:  e.Crtime,
		flags:   Flags(e.Flags),
		rev:     Revision(e.Revision),
	}
	if e.Content != nil {
		// Copy the slice so that the domain Node owns its content and is not
		// affected by mutations to the ent result.
		c := make(Content, len(*e.Content))
		copy(c, *e.Content)
		n.content = c
	}
	return n
}

// contentToBytes returns a pointer suitable for ent's NillableContent setter.
// If content is nil, returns nil; otherwise returns a pointer to a copy.
func contentToBytes(c Content) []byte {
	if c == nil {
		return nil
	}
	b := make([]byte, len(c))
	copy(b, c)
	return b
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

// now is overridable in tests.
var now = time.Now
