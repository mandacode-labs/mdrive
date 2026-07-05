package nodeop

import (
	"context"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/ent"
	entnode "github.com/mandacode-labs/mdrive/ent/node"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// BlockStorage is the data-access contract for nodes.
type BlockStorage interface {
	Read(ctx context.Context, id uuid.UUID) (*node.Node, error)
	Write(ctx context.Context, n *node.Node) error
	Destroy(ctx context.Context, id uuid.UUID) error
}

type blockStorage struct {
	client *ent.Client
}

// Destroy implements [BlockStorage].
func (s *blockStorage) Destroy(ctx context.Context, id uuid.UUID) error {
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

// Read implements [BlockStorage].
func (s *blockStorage) Read(ctx context.Context, id uuid.UUID) (*node.Node, error) {
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

// Write implements [BlockStorage].
func (s *blockStorage) Write(ctx context.Context, n *node.Node) error {
	client := s.client
	if tx, ok := entx.FromContext(ctx); ok {
		client = tx.Client()
	}
	flags := n.Flags()
	err := client.Node.Create().
		SetID(n.ID()).
		SetKind(entKind(n.Kind())).
		SetData(n.Data()).
		SetSize(n.Size()).
		SetDriveID(n.DriveID()).
		SetNlink(n.NLink()).
		SetAtime(n.ATime()).
		SetMtime(n.MTime()).
		SetCtime(n.CTime()).
		SetCrtime(n.CRTime()).
		SetFlags(flags.UInt32()).
		SetRevision(n.Revision().String()).
		OnConflictColumns(entnode.FieldID).
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return err
	}
	n.SetStaleRev(n.Revision())
	return nil
}

func NewSuperOperation(client *ent.Client) BlockStorage {
	return &blockStorage{client: client}
}

func fromEnt(e *ent.Node) *node.Node {
	if e == nil {
		return nil
	}
	rev := node.Revision(e.Revision)
	n := node.NewNode(
		e.ID,
		e.Drv,
		parseNodeKind(e.Kind),
	)
	n.SetData(e.Data)
	n.SetSize(e.Size)
	n.SetNLink(e.Nlink)
	n.SetATime(e.Atime)
	n.SetMTime(e.Mtime)
	n.SetCTime(e.Ctime)
	n.SetCRTime(e.Crtime)
	n.SetFlags(node.Flags(e.Flags))
	n.SetRevision(rev)
	n.SetStaleRev(rev)
	return n
}

func parseNodeKind(s entnode.Kind) node.NodeKind {
	switch s {
	case entnode.KindFile:
		return node.NodeKindFile
	case entnode.KindDirectory:
		return node.NodeKindDirectory
	case entnode.KindSymlink:
		return node.NodeKindSymlink
	case entnode.KindObject:
		return node.NodeKindObject
	case entnode.KindMount:
		return node.NodeKindMount
	default:
		return node.NodeKind(0)
	}
}

func entKind(k node.NodeKind) entnode.Kind {
	switch k {
	case node.NodeKindFile:
		return entnode.KindFile
	case node.NodeKindDirectory:
		return entnode.KindDirectory
	case node.NodeKindSymlink:
		return entnode.KindSymlink
	case node.NodeKindObject:
		return entnode.KindObject
	case node.NodeKindMount:
		return entnode.KindMount
	default:
		return entnode.KindFile
	}
}
