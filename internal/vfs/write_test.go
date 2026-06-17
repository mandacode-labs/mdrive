package vfs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAndCat(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	err := svc.Write(ctx, "user1", "d1", "/data.txt", "hello world")
	require.NoError(t, err)

	raw, err := svc.Cat(ctx, "user1", "d1", "/data.txt")
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(raw))
}
