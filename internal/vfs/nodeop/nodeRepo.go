package nodeop

import (
	"context"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/ent"
	entnode "github.com/mandacode-labs/mdrive/ent/node"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/oklog/ulid/v2"
)

// NodeRepository is the data-access contract for nodes.
type NodeRepository interface {
	Read(ctx context.Context, id uuid.UUID) (*vfs.Node, error)
	Write(ctx context.Context, n *vfs.Node) error
	Destroy(ctx context.Context, id uuid.UUID) error
}

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

func (r *nodeRepo) Read(ctx context.Context, id uuid.UUID) (*vfs.Node, error) {
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

func (r *nodeRepo) Write(ctx context.Context, n *vfs.Node) error {
	client := r.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	flags := n.Flags()
	err := client.Node.Create().
		SetID(n.ID()).
		SetKind(entKind(n.Kind())).
		SetData(n.Data()).
		SetSize(n.Size()).
		SetDriveID(n.Drive().String()).
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

func fromEnt(e *ent.Node) (*vfs.Node, error) {
	if e == nil {
		return nil, errorx.New(errorx.KindNotFound, "node: not found")
	}
	rev := vfs.Revision(e.Revision)
	driveID, err := ulid.Parse(e.DriveID)
	if err != nil {
		return nil, errorx.New(errorx.KindInternal, "invalid drive id")
	}
	kind := parseNodeKind(e.Kind)
	flags := vfs.Flags(e.Flags)

	n := vfs.HydrateNode(
		e.ID,
		driveID,
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

func parseNodeKind(s entnode.Kind) vfs.NodeKind {
	switch s {
	case entnode.KindFile:
		return vfs.NodeKindFile
	case entnode.KindDirectory:
		return vfs.NodeKindDirectory
	case entnode.KindSymlink:
		return vfs.NodeKindSymlink
	case entnode.KindObject:
		return vfs.NodeKindObject
	case entnode.KindMount:
		return vfs.NodeKindMount
	default:
		return vfs.NodeKind(0)
	}
}

func entKind(k vfs.NodeKind) entnode.Kind {
	switch k {
	case vfs.NodeKindFile:
		return entnode.KindFile
	case vfs.NodeKindDirectory:
		return entnode.KindDirectory
	case vfs.NodeKindSymlink:
		return entnode.KindSymlink
	case vfs.NodeKindObject:
		return entnode.KindObject
	case vfs.NodeKindMount:
		return entnode.KindMount
	default:
		return entnode.KindFile
	}
}