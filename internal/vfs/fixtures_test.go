package vfs_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	vfsMocks "github.com/mandacode-labs/mdrive/internal/vfs/mocks"
)

type memNodeRepo struct {
	nodes   map[uuid.UUID]*node.Node
	deleted map[uuid.UUID]bool
}

func newMemNodeRepo() *memNodeRepo {
	return &memNodeRepo{
		nodes:   map[uuid.UUID]*node.Node{},
		deleted: map[uuid.UUID]bool{},
	}
}

func (m *memNodeRepo) get(id uuid.UUID) (*node.Node, bool) {
	if m.deleted[id] {
		return nil, false
	}
	n, ok := m.nodes[id]
	return n, ok
}

func (m *memNodeRepo) save(n *node.Node) {
	m.nodes[n.ID()] = n
}

func (m *memNodeRepo) del(id uuid.UUID) {
	m.deleted[id] = true
	delete(m.nodes, id)
}

func newMockNodeClient(t *testing.T, mem *memNodeRepo) *vfsMocks.NodeClientMock {
	t.Helper()
	mc := vfsMocks.NewNodeClientMock(t)
	mc.EXPECT().GetByID(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id uuid.UUID) (*node.Node, error) {
			if n, ok := mem.get(id); ok {
				return n, nil
			}
			return nil, errorx.New(errorx.KindNotFound, "node: not found")
		},
	).Maybe()
	mc.EXPECT().Link(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, parent *node.Node, name string, child *node.Node) error {
			if err := parent.AddEntry(name, child); err != nil {
				return err
			}
			mem.save(child)
			child.IncNLink()
			return nil
		},
	).Maybe()
	mc.EXPECT().Unlink(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, parent *node.Node, name string) (*node.Node, error) {
			dc, err := parent.ReadDir()
			if err != nil {
				return nil, err
			}
			var childID uuid.UUID
			for _, e := range dc.Entries {
				if e.Name == name {
					childID = e.InodeID
					break
				}
			}
			if childID == uuid.Nil {
				return nil, errorx.New(errorx.KindNotFound, "node: entry not found")
			}
			if err := parent.RemoveEntry(name); err != nil {
				return nil, err
			}
			child, ok := mem.get(childID)
			if !ok {
				return nil, errorx.New(errorx.KindNotFound, "node: not found")
			}
			if child.NLink() > 1 {
				return nil, nil
			}
			mem.del(childID)
			return child, nil
		},
	).Maybe()
	mc.EXPECT().MoveEntry(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, src *node.Node, srcName string, dst *node.Node, dstName string) error {
			dc, err := src.ReadDir()
			if err != nil {
				return err
			}
			var entry *node.DirEntry
			for i := range dc.Entries {
				if dc.Entries[i].Name == srcName {
					entry = &dc.Entries[i]
					break
				}
			}
			if entry == nil {
				return errorx.New(errorx.KindNotFound, "vfs: src entry not found")
			}
			// If dst already has the entry, do not overwrite; the
			// real node.Service.MoveEntry handles overwrite separately.
			if existing, _ := dst.Lookup(dstName); existing != nil {
				return errorx.New(errorx.KindConflict, "node: entry already exists")
			}
			if err := src.RemoveEntry(srcName); err != nil {
				return err
			}
			moved, ok := mem.get(entry.InodeID)
			if !ok {
				return errorx.New(errorx.KindNotFound, "vfs: moved inode not found")
			}
			return dst.AddEntry(dstName, moved)
		},
	).Maybe()
	mc.EXPECT().Save(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, n *node.Node) error {
			mem.save(n)
			return nil
		},
	).Maybe()
	return mc
}

type memDrive struct {
	rootID          uuid.UUID
	storageOverride *drive.Storage
	deletedAt       *time.Time
}

func newMockDriveClient(t *testing.T, m *memDrive) *vfsMocks.DriveClientMock {
	t.Helper()
	dc := vfsMocks.NewDriveClientMock(t)
	dc.EXPECT().GetByID(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ string) (*drive.Drive, error) {
			now := time.Now()
			return drive.NewDrive("d1", "d1", "test", nil, drive.ProviderS3, "owner1", &m.rootID, m.deletedAt, now, now), nil
		},
	).Maybe()
	dc.EXPECT().GetByPublicID(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, id string) (*drive.Drive, error) {
			return dc.GetByID(ctx, id)
		},
	).Maybe()
	dc.EXPECT().GetStorage(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ string) (*drive.Storage, error) {
			if m.storageOverride != nil {
				return m.storageOverride, nil
			}
			return drive.NewStorage("d1", "b", nil, "us-east-1", "a", "s", false), nil
		},
	).Maybe()
	return dc
}

func newMockGarbageRecorder(t *testing.T) *vfsMocks.GarbageRecorderMock {
	t.Helper()
	g := vfsMocks.NewGarbageRecorderMock(t)
	g.EXPECT().RecordGarbage(mock.Anything, mock.Anything).Return(nil).Maybe()
	return g
}

func newMockTxManager(t *testing.T) *vfsMocks.TxManagerMock {
	t.Helper()
	tm := vfsMocks.NewTxManagerMock(t)
	tm.EXPECT().WithTx(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(ctx context.Context) error) error {
			return fn(ctx)
		},
	).Maybe()
	return tm
}

// setupRoot creates a root directory and aligns the drive and node
// repo so the drive's rootID matches the root node's ID.
func setupRoot(t *testing.T) (*node.Node, *memDrive, *memNodeRepo) {
	t.Helper()
	root, err := node.NewDirectory()
	if err != nil {
		t.Fatalf("setupRoot: %v", err)
	}
	driveState := &memDrive{rootID: root.ID()}
	repo := newMemNodeRepo()
	repo.save(root)
	return root, driveState, repo
}

func newTestService(t *testing.T) *vfs.Service {
	svc, _ := newTestServiceWithNode(t)
	return svc
}

func newTestServiceWithNode(t *testing.T) (*vfs.Service, *node.Service) {
	t.Helper()
	_, driveState, repo := setupRoot(t)
	nodeMock := newMockNodeClient(t, repo)
	driveMock := newMockDriveClient(t, driveState)
	garbageMock := newMockGarbageRecorder(t)
	tmMock := newMockTxManager(t)
	return vfs.NewService(vfs.ServiceConfig{
		NodeClient:      nodeMock,
		DriveClient:     driveMock,
		GarbageRecorder: garbageMock,
		TxManager:       tmMock,
	}), nil
}
