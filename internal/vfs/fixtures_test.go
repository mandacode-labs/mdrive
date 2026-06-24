package vfs

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
)

type fakeRepo struct {
	nodes map[uuid.UUID]*node.Node
}

func newFakeRepo() *fakeRepo { return &fakeRepo{nodes: map[uuid.UUID]*node.Node{}} }

var _ node.Repository = (*fakeRepo)(nil)

func (r *fakeRepo) Get(_ context.Context, id uuid.UUID) (*node.Node, error) {
	n, ok := r.nodes[id]
	if !ok {
		return nil, node.ErrNotFound
	}
	c := n.Clone()
	c.SetStaleRev(n.Revision())
	return c, nil
}

func (r *fakeRepo) Save(_ context.Context, n *node.Node) error {
	existing, ok := r.nodes[n.ID()]
	if !ok {
		r.nodes[n.ID()] = n.Clone()
		n.SetStaleRev(n.Revision())
		return nil
	}
	if existing.Revision() != n.StaleRev() {
		return node.ErrRevisionConflict
	}
	r.nodes[n.ID()] = n.Clone()
	n.SetStaleRev(n.Revision())
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(r.nodes, id)
	return nil
}

func (r *fakeRepo) WithTx(_ context.Context, fn func(node.Repository) error) error {
	return fn(r)
}

type fakeDrive struct {
	rootID          uuid.UUID
	storageOverride *drive.Storage
	deletedAt       *time.Time
}

func (d *fakeDrive) Create(_ context.Context, _ string, _ *string, _ string, _ drive.StorageConfig) (*drive.Drive, uuid.UUID, error) {
	return nil, uuid.Nil, nil
}
func (d *fakeDrive) GetByID(_ context.Context, _ string) (*drive.Drive, error) {
	dr := drive.NewDrive("d1", "d1", "test", nil, drive.ProviderS3, "owner1", &d.rootID, d.deletedAt, d.now(), d.now())
	return dr, nil
}
func (d *fakeDrive) GetByPublicID(_ context.Context, _ string) (*drive.Drive, error) {
	return d.GetByID(context.Background(), "")
}
func (d *fakeDrive) GetStorage(_ context.Context, _, _ string) (*drive.Storage, error) {
	if d.storageOverride != nil {
		return d.storageOverride, nil
	}
	return drive.NewStorage("d1", "b", nil, "us-east-1", "a", "s", false), nil
}
func (d *fakeDrive) Update(_ context.Context, _ string, _, _ *string) (*drive.Drive, error) {
	return nil, nil
}
func (d *fakeDrive) Delete(_ context.Context, _ string) error                  { return nil }
func (d *fakeDrive) Restore(_ context.Context, _ string) (*drive.Drive, error) { return nil, nil }
func (d *fakeDrive) ListDeleted(_ context.Context, _ time.Time, _ int) ([]*drive.Drive, error) {
	return nil, nil
}
func (d *fakeDrive) ListByOwner(_ context.Context, _ string) ([]*drive.Drive, error) { return nil, nil }
func (d *fakeDrive) now() time.Time                                                  { return time.Now() }

type fakeStore struct{}

func (s *fakeStore) PutObject(_ context.Context, _, _ string, _ io.Reader) error      { return nil }
func (s *fakeStore) GetObject(_ context.Context, _, _ string) ([]byte, error)         { return nil, nil }
func (s *fakeStore) DeleteObject(_ context.Context, _, _ string) error                { return nil }
func (s *fakeStore) DeleteObjects(_ context.Context, _ string, _ []string) error      { return nil }
func (s *fakeStore) ObjectExists(_ context.Context, _, _ string) (bool, error)        { return true, nil }
func (s *fakeStore) GetObjectSize(_ context.Context, _, _ string) (int64, error)      { return 0, nil }
func (s *fakeStore) GetObjectChecksum(_ context.Context, _, _ string) (string, error) { return "", nil }
func (s *fakeStore) GetPresignedUploadURL(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	return "https://s3.example.com/put", nil
}
func (s *fakeStore) GetPresignedDownloadURL(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	return "https://s3.example.com/get", nil
}

func newTestService() *Service {
	repo := newFakeRepo()
	nodeSvc := node.NewService(repo)
	root, _ := nodeSvc.CreateDirectory(context.Background())
	d := &fakeDrive{rootID: root.ID()}
	return NewService(ServiceConfig{
		Node:  nodeSvc,
		Drive: d,
		Store: &fakeStore{},
	})
}
