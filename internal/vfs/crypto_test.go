package vfs

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	cryptopkg "github.com/mandacode-labs/mdrive/internal/crypto"
)

// newEncryptedTestService builds a vfs.Service whose ContentCipher
// is wired to a real DEKProvider, with the fake drive exposing a
// real wrapped DEK. Returns the service, repo, and the driveID it
// is bound to. The test process owns the master key.
func newEncryptedTestService(t *testing.T) (*Service, *fakeRepo, string, *drive.Storage) {
	t.Helper()
	master := make([]byte, 32)
	for i := range master {
		master[i] = byte(i)
	}
	provider, err := cryptopkg.NewDEKProvider(master)
	if err != nil {
		t.Fatalf("newDEKProvider: %v", err)
	}
	wrapped, err := provider.NewWrappedDEK()
	if err != nil {
		t.Fatalf("NewWrappedDEK: %v", err)
	}
	st := drive.NewStorage("d1", "b", nil, "us-east-1", "a", "s", false, wrapped)

	d := &fakeDrive{rootID: uuid.Nil, storageOverride: st}
	repo := newFakeRepo()
	nodeSvc := node.NewService(repo)
	root, err := nodeSvc.CreateDirectory(context.Background())
	if err != nil {
		t.Fatalf("CreateDirectory: %v", err)
	}
	d.rootID = root.ID()

	cipher := func(ctx context.Context, driveID string) (*cryptopkg.NodeCipher, error) {
		wrapped := st.WrappedDEK()
		dek, err := provider.Unwrap(wrapped)
		if err != nil {
			return nil, err
		}
		return cryptopkg.NewNodeCipher(dek)
	}
	svc := NewService(nodeSvc, d, &fakeUser{}, &fakeStore{}, &fakePerm{}, nil, nil, cipher, nil)
	return svc, repo, "d1", st
}

func TestEnvelopeWriteCatRoundTrip(t *testing.T) {
	svc, repo, driveID, _ := newEncryptedTestService(t)
	ctx := context.Background()

	if err := svc.Write(ctx, "owner1", driveID, "secret.txt", "hello world"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Assert stored content is ciphertext immediately after Write,
	// before Cat mutates the in-memory node back to plaintext.
	for _, n := range repo.nodes {
		if n.IsFile() {
			ct := []byte(n.Content())
			if len(ct) == 0 {
				t.Fatalf("stored content is empty")
			}
			if string(ct) == `"hello world"` {
				t.Fatalf("stored content is plaintext FileContent JSON; want ciphertext")
			}
		}
	}
	got, err := svc.Cat(ctx, "owner1", driveID, "secret.txt")
	if err != nil {
		t.Fatalf("Cat: %v", err)
	}
	if string(got) != "hello world" {
		t.Fatalf("Cat content = %q, want %q", got, "hello world")
	}
}

func TestEnvelopeAADMismatchSurfaces(t *testing.T) {
	// Confirm that tampering with the stored ciphertext causes
	// the next Cat to fail: AES-GCM authentication rejects the
	// modified bytes. This is the property that AAD also protects.
	svc, repo, driveID, _ := newEncryptedTestService(t)
	ctx := context.Background()

	if err := svc.Write(ctx, "owner1", driveID, "secret.txt", "hello world"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Flip a single bit in the stored ciphertext.
	for _, n := range repo.nodes {
		if n.IsFile() {
			ct := []byte(n.Content())
			ct[0] ^= 0x01
			n.SetContent(ct)
			break
		}
	}
	if _, err := svc.Cat(ctx, "owner1", driveID, "secret.txt"); err == nil {
		t.Fatalf("Cat on tampered ciphertext should fail")
	}
}

func TestEnvelopeNoCipherStoresPlaintext(t *testing.T) {
	// When contentCipher is nil (the default in unit tests),
	// the vfs must store plaintext, so existing tests that
	// don't care about encryption keep working.
	svc := newTestService()
	if err := svc.Write(context.Background(), "owner1", "d1", "plain.txt", "hi"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := svc.Cat(context.Background(), "owner1", "d1", "plain.txt")
	if err != nil {
		t.Fatalf("Cat: %v", err)
	}
	if string(got) != "hi" {
		t.Fatalf("plaintext round-trip failed: got %q", got)
	}
}
