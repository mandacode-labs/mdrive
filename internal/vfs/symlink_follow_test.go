package vfs

import (
	"github.com/mandacode-labs/mdrive/internal/errorx"
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
		NodeClient:   nodeSvc,
		DriveClient:  &fakeDrive{rootID: root.ID()},
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
		NodeClient:   nodeSvc,
		DriveClient:  &fakeDrive{rootID: root.ID()},
	})

	_, err = svc.Symlink(ctx, "d1", "/loop", "/loop")
	require.NoError(t, err)

	_, err = svc.Resolve(ctx, "d1", "/loop")
	assertKind(t, err, errorx.KindBadRequest)
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
		NodeClient:   nodeSvc,
		DriveClient:  &fakeDrive{rootID: root.ID()},
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

// TestSymlinkDanglingTarget: a symlink whose target does not exist
// is created without error (POSIX allows dangling links), and
// Resolve surfaces ErrNotFound when following it.
func TestSymlinkDanglingTarget(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	nodeSvc := node.NewService(repo)
	root, err := nodeSvc.CreateDirectory(ctx)
	require.NoError(t, err)
	svc := NewService(ServiceConfig{
		NodeClient:  nodeSvc,
		DriveClient: &fakeDrive{rootID: root.ID()},
	})

	// Create a symlink to a target that does not exist.
	_, err = svc.Symlink(ctx, "d1", "/does-not-exist", "/dangling")
	require.NoError(t, err, "dangling symlink must be creatable")

	// Readlink returns the target unchanged.
	ls, err := svc.Lstat(ctx, "d1", "/dangling")
	require.NoError(t, err)
	target, err := ls.Node.Readlink()
	require.NoError(t, err)
	assert.Equal(t, "/does-not-exist", target)

	// Resolve follows the link and surfaces ErrNotFound.
	_, err = svc.Resolve(ctx, "d1", "/dangling")
	assertKind(t, err, errorx.KindNotFound)
}

// TestSymlinkRelativeTarget: a symlink with a relative target is
// resolved relative to the symlink's parent directory, matching
// POSIX symlink(2) semantics.
func TestSymlinkRelativeTarget(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	nodeSvc := node.NewService(repo)
	root, err := nodeSvc.CreateDirectory(ctx)
	require.NoError(t, err)
	svc := NewService(ServiceConfig{
		NodeClient:  nodeSvc,
		DriveClient: &fakeDrive{rootID: root.ID()},
	})

	_, err = svc.Mkdir(ctx, "d1", "/data")
	require.NoError(t, err)
	_, err = svc.Touch(ctx, "d1", "/data/sibling.txt")
	require.NoError(t, err)
	require.NoError(t, svc.Write(ctx, "d1", "/data/sibling.txt", "sibling"))

	// Create a symlink with a relative target from /data/ to
	// /data/sibling.txt. POSIX resolves this relative to the
	// symlink's parent directory (/data), so 'sibling.txt' is
	// interpreted as /data/sibling.txt.
	_, err = svc.Symlink(ctx, "d1", "sibling.txt", "/data/link")
	require.NoError(t, err)

	res, err := svc.Resolve(ctx, "d1", "/data/link")
	require.NoError(t, err)
	assert.Equal(t, node.NodeTypeFile, res.Node.Type(),
		"relative symlink must resolve to the sibling file")

	content, err := svc.Cat(ctx, "d1", "/data/link")
	require.NoError(t, err)
	assert.Equal(t, "sibling", string(content))
}
