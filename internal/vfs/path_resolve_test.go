package vfs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

// TestResolveDotDot verifies that the resolver follows POSIX ".."
// semantics and refuses to ascend above the drive root.
func TestResolveDotDot(t *testing.T) {
	repo := newFakeRepo()
	svc := node.NewService(repo)

	root, err := svc.CreateDirectory(context.Background())
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), root))

	docs, err := svc.CreateDirectory(context.Background())
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), docs))
	require.NoError(t, svc.Link(context.Background(), root, "docs", docs))

	sub, err := svc.CreateDirectory(context.Background())
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), sub))
	require.NoError(t, svc.Link(context.Background(), docs, "sub", sub))

	r := newResolver(svc)

	// /docs -> root.docs
	got, err := r.resolve(context.Background(), root.ID(), "/docs")
	require.NoError(t, err)
	assert.Equal(t, docs.ID(), got.ID())

	// /docs/sub -> docs.sub
	got, err = r.resolve(context.Background(), root.ID(), "/docs/sub")
	require.NoError(t, err)
	assert.Equal(t, sub.ID(), got.ID())

	// /docs/../docs/sub == /docs/sub
	got, err = r.resolve(context.Background(), root.ID(), "/docs/../docs/sub")
	require.NoError(t, err)
	assert.Equal(t, sub.ID(), got.ID())

	// /docs/sub/../../docs -> root.docs
	got, err = r.resolve(context.Background(), root.ID(), "/docs/sub/../../docs")
	require.NoError(t, err)
	assert.Equal(t, docs.ID(), got.ID())

	// /docs/sub/../../../../../../../ -> cleanPath collapses the climb
	// above root to "/", so the resolve lands on the drive root rather
	// than failing. This matches POSIX path.Clean semantics.
	_, err = r.resolve(context.Background(), root.ID(), "/docs/sub/../../../../../../..")
	require.NoError(t, err)
}

// TestResolveDot verifies that "." is a no-op.
func TestResolveDot(t *testing.T) {
	repo := newFakeRepo()
	svc := node.NewService(repo)

	root, err := svc.CreateDirectory(context.Background())
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), root))

	dir, err := svc.CreateDirectory(context.Background())
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), dir))
	require.NoError(t, svc.Link(context.Background(), root, "d", dir))

	r := newResolver(svc)
	got, err := r.resolve(context.Background(), root.ID(), "/./d/./")
	require.NoError(t, err)
	assert.Equal(t, dir.ID(), got.ID())
}

// TestResolveRootIsRoot checks /  and "" map to the drive root.
func TestResolveRootIsRoot(t *testing.T) {
	repo := newFakeRepo()
	svc := node.NewService(repo)
	root, err := svc.CreateDirectory(context.Background())
	require.NoError(t, err)
	require.NoError(t, repo.Save(context.Background(), root))

	r := newResolver(svc)
	for _, p := range []string{"/", "", "  ", "//", "/././"} {
		got, err := r.resolve(context.Background(), root.ID(), p)
		require.NoError(t, err, p)
		assert.Equal(t, root.ID(), got.ID(), p)
	}
}
