package vfs

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
)

// fakeNode is a nodeClient that stores nodes in memory.
type fakeNode struct {
	nodes map[uuid.UUID]*node.Node
}

func newFakeNode() *fakeNode { return &fakeNode{nodes: map[uuid.UUID]*node.Node{}} }

func (f *fakeNode) CreateFile(_ context.Context, content string) (*node.Node, error) {
	n, _ := node.NewFile(content)
	f.nodes[n.ID()] = n
	return n, nil
}
func (f *fakeNode) CreateDirectory(_ context.Context) (*node.Node, error) {
	n, _ := node.NewDirectory()
	f.nodes[n.ID()] = n
	return n, nil
}
func (f *fakeNode) CreateSymlink(_ context.Context, target string) (*node.Node, error) {
	n, _ := node.NewSymlink(target)
	f.nodes[n.ID()] = n
	return n, nil
}
func (f *fakeNode) CreateObject(_ context.Context, c node.ObjectContent, s int64) (*node.Node, error) {
	n, _ := node.NewObject(c, s)
	f.nodes[n.ID()] = n
	return n, nil
}
func (f *fakeNode) Link(_ context.Context, parent *node.Node, name string, child *node.Node) error {
	return parent.AddEntry(name, child)
}
func (f *fakeNode) Unlink(_ context.Context, parent *node.Node, name string) error {
	return parent.RemoveEntry(name)
}
func (f *fakeNode) GetByID(_ context.Context, id uuid.UUID) (*node.Node, error) {
	n, ok := f.nodes[id]
	if !ok {
		return nil, node.ErrNotFound
	}
	return n, nil
}
func (f *fakeNode) Delete(_ context.Context, id uuid.UUID) error {
	delete(f.nodes, id)
	return nil
}
func (f *fakeNode) WithTx(_ context.Context, fn func(tx *node.Service) error) error {
	return fn(nil)
}

// fakeDrive is a driveClient stub.
type fakeDrive struct{ rootID uuid.UUID }

func (d *fakeDrive) Create(_ context.Context, _ string, _ *string, _ string, _ drive.StorageConfig) (*drive.Drive, uuid.UUID, error) {
	return nil, uuid.Nil, nil
}
func (d *fakeDrive) GetByID(_ context.Context, _ string) (*drive.Drive, error) {
	now := time.Now()
	return drive.NewDrive("d1", "d1", "test", nil, drive.ProviderS3, "owner1", &d.rootID, now, now), nil
}
func (d *fakeDrive) GetByPublicID(_ context.Context, _ string) (*drive.Drive, error) {
	return d.GetByID(nil, "")
}
func (d *fakeDrive) GetStorage(_ context.Context, _ string) (*drive.Storage, error) {
	return drive.NewStorage("d1", "b", nil, "us-east-1", "a", "s", false), nil
}
func (d *fakeDrive) Update(_ context.Context, _ string, _, _ *string) (*drive.Drive, error) {
	return nil, nil
}
func (d *fakeDrive) Delete(_ context.Context, _ string) error { return nil }
func (d *fakeDrive) ListByOwner(_ context.Context, _ string) ([]*drive.Drive, error) {
	return nil, nil
}

// fakeUser is a userClient stub.
type fakeUser struct{}

func (u *fakeUser) UpsertFromOIDC(_ context.Context, _ *user.CreateCommand) (*user.User, error) {
	return nil, nil
}
func (u *fakeUser) GetByID(_ context.Context, _ string) (*user.User, error) { return nil, nil }
func (u *fakeUser) GetByPublicID(_ context.Context, _ string) (*user.User, error) {
	return nil, nil
}
func (u *fakeUser) GetByProviderID(_ context.Context, _, _ string) (*user.User, error) {
	return nil, nil
}
func (u *fakeUser) Update(_ context.Context, _ *user.User) (*user.User, error) {
	return nil, nil
}
func (u *fakeUser) Exists(_ context.Context, _ string) (bool, error) { return true, nil }

// fakePerm is a permClient that grants everything.
type fakePerm struct{}

func (p *fakePerm) Check(_ context.Context, _, _, _, _ string) (bool, error) { return true, nil }
func (p *fakePerm) Grant(_ context.Context, _, _, _, _ string) error         { return nil }

// fakeStore is a Storage stub.
type fakeStore struct{}

func (s *fakeStore) PutObject(_ context.Context, _, _ string, _ io.Reader, _ int64) error { return nil }
func (s *fakeStore) GetObject(_ context.Context, _, _ string) ([]byte, error) { return nil, nil }
func (s *fakeStore) DeleteObject(_ context.Context, _, _ string) error       { return nil }
func (s *fakeStore) DeleteObjects(_ context.Context, _ string, _ []string) error { return nil }
func (s *fakeStore) ObjectExists(_ context.Context, _, _ string) (bool, error) { return true, nil }
func (s *fakeStore) GetObjectSize(_ context.Context, _, _ string) (int64, error) { return 0, nil }
func (s *fakeStore) GetObjectChecksum(_ context.Context, _, _ string) (string, error) { return "", nil }
func (s *fakeStore) GetPresignedUploadURL(_ context.Context, _, _, _ string, _ int64, _ string, _ time.Duration) (string, error) {
	return "", nil
}
func (s *fakeStore) GetPresignedDownloadURL(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	return "", nil
}

// testService creates a Service wired to fakes.
func testService() *Service {
	n := newFakeNode()
	// Create a root directory for drive "d1".
	root, _ := n.CreateDirectory(context.Background())
	d := &fakeDrive{rootID: root.ID()}
	return NewService(n, d, &fakeUser{}, &fakeStore{}, &fakePerm{})
}

func TestMkdir(t *testing.T) {
	svc := testService()
	ctx := context.Background()

	n, err := svc.Mkdir(ctx, "user1", "d1", "/foo")
	if err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if !n.IsDir() {
		t.Error("expected directory")
	}

	// Verify it shows up in root listing.
	dc, err := svc.Ls(ctx, "user1", "d1", "/")
	if err != nil {
		t.Fatalf("Ls: %v", err)
	}
	if len(dc.Entries) != 1 || dc.Entries[0].Name != "foo" {
		t.Errorf("expected ['foo'], got %+v", dc.Entries)
	}
}

func TestMkdirNested(t *testing.T) {
	svc := testService()
	ctx := context.Background()

	// Create parents first (like `mkdir -p` or two separate calls).
	_, err := svc.Mkdir(ctx, "user1", "d1", "/a")
	if err != nil {
		t.Fatalf("Mkdir /a: %v", err)
	}
	_, err = svc.Mkdir(ctx, "user1", "d1", "/a/b")
	if err != nil {
		t.Fatalf("Mkdir /a/b: %v", err)
	}
	dc, err := svc.Ls(ctx, "user1", "d1", "/a")
	if err != nil {
		t.Fatalf("Ls /a: %v", err)
	}
	if len(dc.Entries) != 1 || dc.Entries[0].Name != "b" {
		t.Fatal("expected /a/b to exist")
	}
}

func TestTouch(t *testing.T) {
	svc := testService()
	ctx := context.Background()

	n, err := svc.Touch(ctx, "user1", "d1", "/hello.txt")
	if err != nil {
		t.Fatalf("Touch: %v", err)
	}
	if !n.IsFile() {
		t.Error("expected file")
	}

	// Read it back.
	raw, err := svc.Cat(ctx, "user1", "d1", "/hello.txt")
	if err != nil {
		t.Fatalf("Cat: %v", err)
	}
	if string(raw) != "" {
		t.Errorf("expected empty file, got %q", raw)
	}
}

func TestWriteAndCat(t *testing.T) {
	svc := testService()
	ctx := context.Background()

	if err := svc.Write(ctx, "user1", "d1", "/data.txt", "hello world"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	raw, err := svc.Cat(ctx, "user1", "d1", "/data.txt")
	if err != nil {
		t.Fatalf("Cat: %v", err)
	}
	if string(raw) != "hello world" {
		t.Errorf("expected 'hello world', got %q", raw)
	}
}

func TestStat(t *testing.T) {
	svc := testService()
	ctx := context.Background()

	_, _ = svc.Touch(ctx, "user1", "d1", "/x")
	n, err := svc.Stat(ctx, "user1", "d1", "/x")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !n.IsFile() {
		t.Error("expected file")
	}
	if n.Size() != 0 {
		t.Errorf("expected size 0, got %d", n.Size())
	}
}

func TestRm(t *testing.T) {
	svc := testService()
	ctx := context.Background()

	_, _ = svc.Touch(ctx, "user1", "d1", "/x")
	_, _ = svc.Touch(ctx, "user1", "d1", "/y")

	if err := svc.Rm(ctx, "user1", "d1", []string{"/x", "/y"}, false); err != nil {
		t.Fatalf("Rm: %v", err)
	}
	// Both should be gone.
	_, err := svc.Stat(ctx, "user1", "d1", "/x")
	if err == nil {
		t.Error("expected /x to be removed")
	}
}

func TestRmRecursive(t *testing.T) {
	svc := testService()
	ctx := context.Background()

	_, _ = svc.Mkdir(ctx, "user1", "d1", "/dir")
	_, _ = svc.Touch(ctx, "user1", "d1", "/dir/a")
	_, _ = svc.Touch(ctx, "user1", "d1", "/dir/b")

	if err := svc.Rm(ctx, "user1", "d1", []string{"/dir"}, true); err != nil {
		t.Fatalf("Rm -r: %v", err)
	}
	_, err := svc.Stat(ctx, "user1", "d1", "/dir")
	if err == nil {
		t.Error("expected /dir to be removed")
	}
}

func TestRmDirWithoutRecursive(t *testing.T) {
	svc := testService()
	ctx := context.Background()

	_, _ = svc.Mkdir(ctx, "user1", "d1", "/dir")
	err := svc.Rm(ctx, "user1", "d1", []string{"/dir"}, false)
	if err == nil {
		t.Fatal("expected error removing directory without -r")
	}
}

func TestMv(t *testing.T) {
	svc := testService()
	ctx := context.Background()

	_, _ = svc.Mkdir(ctx, "user1", "d1", "/dst")
	_, _ = svc.Touch(ctx, "user1", "d1", "/x")

	if err := svc.Mv(ctx, "user1", "d1", []string{"/x"}, "d1", "/dst/x"); err != nil {
		t.Fatalf("Mv: %v", err)
	}
	_, err := svc.Stat(ctx, "user1", "d1", "/x")
	if err == nil {
		t.Error("expected /x to be moved")
	}
	_, err = svc.Stat(ctx, "user1", "d1", "/dst/x")
	if err != nil {
		t.Errorf("expected /dst/x to exist: %v", err)
	}
}

func TestSymlink(t *testing.T) {
	svc := testService()
	ctx := context.Background()

	_, _ = svc.Touch(ctx, "user1", "d1", "/target")
	n, err := svc.Symlink(ctx, "user1", "d1", "/target", "/link")
	if err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if !n.IsSymlink() {
		t.Error("expected symlink")
	}
	target, _ := n.ReadSymlink()
	if target != "/target" {
		t.Errorf("expected target /target, got %q", target)
	}
	// Cat follows the symlink? No, Cat on a symlink returns the target path.
	raw, _ := svc.Cat(ctx, "user1", "d1", "/link")
	if string(raw) != "/target" {
		t.Errorf("expected cat '/target', got %q", raw)
	}
}
