package vfs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// TestSymlinkFollowAbsoluteTarget: Resolve follows an absolute
// symlink target to the actual target node (POSIX stat(2)).
func TestSymlinkFollowAbsoluteTarget(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	nodeSvc := node.NewService(repo)
	root, err := nodeSvc.CreateDirectory(ctx)
	require.NoError(t, err)
	svc := NewService(ServiceConfig{
		Node:  nodeSvc,
		Drive: &fakeDrive{rootID: root.ID()},
		Store: &fakeStore{},
	})

	_, err = svc.Mkdir(ctx, "d1", "/data")
	require.NoError(t, err)
	_, err = svc.Touch(ctx, "d1", "/data/target.txt")
	require.NoError(t, err)
	require.NoError(t, svc.Write(ctx, "d1", "/data/target.txt", "hello"))

	_, err = svc.Symlink(ctx, "d1", "/data/target.txt", "/link-to-target")
	require.NoError(t, err)

	// Resolve follows the symlink and returns the target node.
	res, err := svc.Resolve(ctx, "d1", "/link-to-target")
	require.NoError(t, err)
	assert.Equal(t, node.NodeTypeFile, res.Node.Type(),
		"Resolve must follow symlink to /data/target.txt (a file)")

	// Cat follows the symlink and returns the file's content.
	content, err := svc.Cat(ctx, "d1", "/link-to-target")
	require.NoError(t, err)
	assert.Equal(t, "hello", string(content))

	// Lstat does NOT follow: returns the symlink itself.
	ls, err := svc.Lstat(ctx, "d1", "/link-to-target")
	require.NoError(t, err)
	assert.Equal(t, node.NodeTypeSymlink, ls.Node.Type(),
		"Lstat must return the symlink itself, not its target")
}

// TestSymlinkCycle: a self-referencing symlink chain surfaces
// ErrSymlinkCycle (POSIX ELOOP).
func TestSymlinkCycle(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	nodeSvc := node.NewService(repo)
	root, err := nodeSvc.CreateDirectory(ctx)
	require.NoError(t, err)
	svc := NewService(ServiceConfig{
		Node:  nodeSvc,
		Drive: &fakeDrive{rootID: root.ID()},
		Store: &fakeStore{},
	})

	_, err = svc.Symlink(ctx, "d1", "/loop", "/loop")
	require.NoError(t, err)

	_, err = svc.Resolve(ctx, "d1", "/loop")
	assert.ErrorIs(t, err, node.ErrSymlinkCycle)
}

// TestSymlinkChain: a chain of symlinks follows all hops until
// the final target.
func TestSymlinkChain(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	nodeSvc := node.NewService(repo)
	root, err := nodeSvc.CreateDirectory(ctx)
	require.NoError(t, err)
	svc := NewService(ServiceConfig{
		Node:  nodeSvc,
		Drive: &fakeDrive{rootID: root.ID()},
		Store: &fakeStore{},
	})

	_, err = svc.Touch(ctx, "d1", "/real.txt")
	require.NoError(t, err)
	require.NoError(t, svc.Write(ctx, "d1", "/real.txt", "real"))

	_, err = svc.Symlink(ctx, "d1", "/real.txt", "/link1")
	require.NoError(t, err)
	_, err = svc.Symlink(ctx, "d1", "/link1", "/link2")
	require.NoError(t, err)
	_, err = svc.Symlink(ctx, "d1", "/link2", "/link3")
	require.NoError(t, err)

	res, err := svc.Resolve(ctx, "d1", "/link3")
	require.NoError(t, err)
	assert.Equal(t, node.NodeTypeFile, res.Node.Type(),
		"Resolve must follow the chain to /real.txt")

	content, err := svc.Cat(ctx, "d1", "/link3")
	require.NoError(t, err)
	assert.Equal(t, "real", string(content))
}
