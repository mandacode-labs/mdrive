package vfs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/node"
)

func newLoggedService(t *testing.T) (*Service, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	svc, _ := newTestServiceWithNode()
	svc.Logger = slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return svc, buf
}

func newLoggedServiceAndNode(t *testing.T) (*Service, *node.Service, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	svc, nodeSvc := newTestServiceWithNode()
	svc.Logger = slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return svc, nodeSvc, buf
}

func TestVFS_Mkdir_LogsNoMessage(t *testing.T) {
	// Sanity: a basic op without multi-step semantics does not
	// produce a vfs.* log. This guards against accidentally
	// logging on every op (high signal-to-noise is the goal).
	ctx := context.Background()
	svc, buf := newLoggedService(t)
	_, err := svc.Mkdir(ctx, "d1", "/sub")
	require.NoError(t, err)
	assert.False(t, strings.Contains(buf.String(), "vfs.mkdir"),
		"plain mkdir should not log; signal is reserved for multi-step ops")
}

func TestVFS_Mv_LogsCompleted(t *testing.T) {
	ctx := context.Background()
	svc, nodeSvc, buf := newLoggedServiceAndNode(t)

	rootID, err := svc.rootNodeID(ctx, "d1")
	require.NoError(t, err)
	root, err := nodeSvc.GetByID(ctx, rootID)
	require.NoError(t, err)

	f, err := nodeSvc.CreateFile(ctx, "hello")
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(ctx, root, "src.txt", f))

	require.NoError(t, svc.Mv(ctx, "d1", []string{"/src.txt"}, "d1", "/dst.txt"))

	out := buf.String()
	assert.Contains(t, out, "vfs.mv.completed")
	assert.Contains(t, out, "tombstoned=0")
}

func TestVFS_Mv_LogsErrorOnTombstoneFailure(t *testing.T) {
	// Constructed indirectly: if the source node is the same
	// object type as the destination, MoveEntry succeeds and
	// the resulting nlink==1 entry produces a tombstone ref.
	// Build: file "f.txt" -> /obj (object), with an existing
	// /obj already in place, then run mv. The resulting call
	// to InsertTombstones is what we want to fail.
	ctx := context.Background()
	svc, nodeSvc, buf := newLoggedServiceAndNode(t)

	rootID, err := svc.rootNodeID(ctx, "d1")
	require.NoError(t, err)
	root, err := nodeSvc.GetByID(ctx, rootID)
	require.NoError(t, err)

	// /obj: existing object target.
	dst, err := nodeSvc.CreateObject(ctx, node.ObjectContent{Bucket: "b", Key: "k-old"}, 4)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(ctx, root, "obj", dst))

	// /f.txt: object source, same type as /obj, so MoveEntry succeeds.
	src, err := nodeSvc.CreateObject(ctx, node.ObjectContent{Bucket: "b", Key: "k-new"}, 4)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(ctx, root, "f.txt", src))

	svc.GarbageRecorder = &fakeGC{err: errors.New("kafka down")}
	err = svc.Mv(ctx, "d1", []string{"/f.txt"}, "d1", "/obj")
	require.Error(t, err)

	out := buf.String()
	assert.Contains(t, out, "vfs.mv.tombstone_failed")
	assert.Contains(t, out, "level=ERROR")
}

func TestVFS_Rm_LogsCompleted(t *testing.T) {
	ctx := context.Background()
	svc, nodeSvc, buf := newLoggedServiceAndNode(t)

	rootID, err := svc.rootNodeID(ctx, "d1")
	require.NoError(t, err)
	root, err := nodeSvc.GetByID(ctx, rootID)
	require.NoError(t, err)

	// Empty file, not a directory: rm without -r succeeds.
	f, err := nodeSvc.CreateFile(ctx, "")
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(ctx, root, "f.txt", f))

	require.NoError(t, svc.Rm(ctx, "d1", []string{"/f.txt"}, false))
	assert.Contains(t, buf.String(), "vfs.rm.completed")
}

func TestVFS_Rm_LogsErrorOnTombstoneFailure(t *testing.T) {
	ctx := context.Background()
	svc, nodeSvc, buf := newLoggedServiceAndNode(t)

	rootID, err := svc.rootNodeID(ctx, "d1")
	require.NoError(t, err)
	root, err := nodeSvc.GetByID(ctx, rootID)
	require.NoError(t, err)

	obj, err := nodeSvc.CreateObject(ctx, node.ObjectContent{Bucket: "b", Key: "k"}, 4)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(ctx, root, "obj", obj))

	svc.GarbageRecorder = &fakeGC{err: errors.New("kafka down")}
	require.Error(t, svc.Rm(ctx, "d1", []string{"/obj"}, false))

	out := buf.String()
	assert.Contains(t, out, "vfs.rm.tombstone_failed")
	assert.Contains(t, out, "level=ERROR")
}

func TestVFS_Mount_LogsCreated(t *testing.T) {
	ctx := context.Background()
	svc, buf := newLoggedService(t)
	require.NoError(t, svc.Mount(ctx, "d1", "/sub", "d2"))
	assert.Contains(t, buf.String(), "vfs.mount.created")
}

func TestVFS_Unmount_LogsCompleted(t *testing.T) {
	ctx := context.Background()
	svc, nodeSvc, buf := newLoggedServiceAndNode(t)

	rootID, err := svc.rootNodeID(ctx, "d1")
	require.NoError(t, err)
	root, err := nodeSvc.GetByID(ctx, rootID)
	require.NoError(t, err)

	mount, err := nodeSvc.CreateMount(ctx, "d2")
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(ctx, root, "m", mount))

	require.NoError(t, svc.Unmount(ctx, "d1", "/m"))
	out := buf.String()
	assert.Contains(t, out, "vfs.unmount.completed")
	assert.Contains(t, out, "source_drive=d2")
}

type fakeGC struct {
	err error
}

func (g *fakeGC) RecordGarbage(_ context.Context, _ []GarbageRef) error {
	return g.err
}
