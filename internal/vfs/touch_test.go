package vfs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTouch(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	n, err := svc.Touch(ctx, "user1", "d1", "/hello.txt")
	require.NoError(t, err)
	assert.True(t, n.IsFile())

	raw, err := svc.Cat(ctx, "user1", "d1", "/hello.txt")
	require.NoError(t, err)
	assert.Empty(t, string(raw))
}
