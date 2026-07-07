package nodeop

import (
	"context"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/ent"
	entnode "github.com/mandacode-labs/mdrive/ent/node"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/fs"
)

// NodeRepository is the data-access contract for nodes.
type NodeRepository interface {
	Read(ctx context.Context, id uuid.UUID) (*fs.Node, error)
	Write(ctx context.Context, n *fs.Node) error
	Destroy(ctx context.Context, id uuid.UUID) error
}

// nodeRepo is the ent-backed impl.
type nodeRepo struct {
	client *ent.Client
}

func (r *nodeRepo) Destroy(ctx context.Context, id uuid.UUID) error {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	err := client.Node.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return errorx.New(errorx.KindNotFound, "node: not found")
		}
		return errorx.Wrap(err, "failed to delete node", errorx.KindInternal)
	}
	return nil
}

func (r *nodeRepo) Read(ctx context.Context, id uuid.UUID) (*fs.Node, error) {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	e, err := client.Node.Query().Where(entnode.IDEQ(id)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errorx.New(errorx.KindNotFound, "node: not found")
		}
		return nil, errorx.Wrap(err, "failed to read node", errorx.KindInternal)
	}
	return fromEnt(e)
}

func (r *nodeRepo) Write(ctx context.Context, n *fs.Node) error {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	flags := n.Flags()
	err := client.Node.Create().
		SetID(n.ID()).
		SetSuperblockID(n.SuperblockID()).
		SetKind(entKind(n.Kind())).
		SetData(n.Data()).
		SetSize(n.Size()).
		SetNlink(n.NLink()).
		SetAtime(n.ATime()).
		SetMtime(n.MTime()).
		SetCtime(n.CTime()).
		SetBtime(n.BTime()).
		SetFlags(flags.UInt32()).
		SetRevision(n.Revision().String()).
		OnConflictColumns(entnode.FieldID).
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return errorx.Wrap(err, "failed to write node", errorx.KindInternal)
	}
	return nil
}

func NewNodeRepository(client *ent.Client) NodeRepository {
	return &nodeRepo{client: client}
}

var _ NodeRepository = (*nodeRepo)(nil)

func fromEnt(e *ent.Node) (*fs.Node, error) {
	if e == nil {
		return nil, errorx.New(errorx.KindNotFound, "node: not found")
	}
	rev := fs.Revision(e.Revision)
	kind := parseNodeKind(e.Kind)
	flags := fs.Flags(e.Flags)

	n := fs.HydrateNode(
		e.ID,
		e.SuperblockID,
		kind,
		e.Size,
		e.Nlink,
		e.Data,
		e.Atime,
		e.Mtime,
		e.Ctime,
		e.Btime,
		flags,
		rev,
		rev,
	)
	return n, nil
}

func parseNodeKind(s entnode.Kind) fs.NodeKind {
	switch s {
	case entnode.KindFile:
		return fs.NodeKindFile
	case entnode.KindDirectory:
		return fs.NodeKindDirectory
	case entnode.KindSymlink:
		return fs.NodeKindSymlink
	case entnode.KindObject:
		return fs.NodeKindObject
	case entnode.KindMount:
		return fs.NodeKindMount
	default:
		return fs.NodeKind(0)
	}
}

func entKind(k fs.NodeKind) entnode.Kind {
	switch k {
	case fs.NodeKindFile:
		return entnode.KindFile
	case fs.NodeKindDirectory:
		return entnode.KindDirectory
	case fs.NodeKindSymlink:
		return entnode.KindSymlink
	case fs.NodeKindObject:
		return entnode.KindObject
	case fs.NodeKindMount:
		return entnode.KindMount
	default:
		return entnode.KindFile
	}
}
