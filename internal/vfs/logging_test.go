package vfs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/logx"
)

// captureLogs installs a buffer-backed JSON logger as
// slog.Default for the test duration, then restores the previous
// default on cleanup. vfs logs flow through slog.Default, so
// the buffer captures every vfs log line.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	logx.New(logx.Config{Env: "test", Level: "debug", Format: "json", Writer: buf})
	return buf
}

func newLoggedService(t *testing.T) (*Service, *bytes.Buffer) {
	t.Helper()
	buf := captureLogs(t)
	svc, _ := newTestServiceWithNode()
	return svc, buf
}

func newLoggedServiceAndNode(t *testing.T) (*Service, *node.Service, *bytes.Buffer) {
	t.Helper()
	buf := captureLogs(t)
	svc, nodeSvc := newTestServiceWithNode()
	return svc, nodeSvc, buf
}

// findLine returns the first log line whose msg matches msg.
// Returns nil if no such line is in the buffer. The lines are
// JSON objects with stable key order; this is the standard
// test hook for asserting on individual log lines.
func findLine(t *testing.T, raw, msg string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(strings.TrimRight(raw, "\n"), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &m),
			"log line must be valid JSON: %q", line)
		if m["msg"] == msg {
			return m
		}
	}
	return nil
}

func TestVFSMkdirLogsEnterOk(t *testing.T) {
	// Every op now emits a vfs.* debug log on enter/ok at log
	// level=debug, so the operator can trace any single op in
	// production by raising log_level. The previous "high signal
	// to noise" rule no longer applies: signal is the trace, the
	// cost is one debug log per op, and log_level=info prod does
	// not see them.
	ctx := context.Background()
	svc, buf := newLoggedService(t)
	_, err := svc.Mkdir(ctx, "d1", "/sub")
	require.NoError(t, err)
	assert.True(t, strings.Contains(buf.String(), "vfs.mkdir.enter"),
		"vfs.mkdir.enter must be present at debug level")
}

func TestVFSMvLogsCompleted(t *testing.T) {
	ctx := context.Background()
	svc, nodeSvc, buf := newLoggedServiceAndNode(t)

	rootID, err := svc.GetRootNodeID(ctx, "d1")
	require.NoError(t, err)
	root, err := nodeSvc.GetByID(ctx, rootID)
	require.NoError(t, err)

	f, err := nodeSvc.CreateFile(ctx, "hello")
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(ctx, root, "src.txt", f))

	require.NoError(t, svc.Mv(ctx, "d1", []string{"/src.txt"}, "d1", "/dst.txt"))

	line := findLine(t, buf.String(), "vfs.mv.completed")
	require.NotNil(t, line, "expected vfs.mv.completed line in %s", buf.String())
	assert.Equal(t, float64(0), line["tombstoned"])
}

func TestVFSMvLogsErrorOnTombstoneFailure(t *testing.T) {
	// Constructed indirectly: if the source node is the same
	// object type as the destination, MoveEntry succeeds and
	// the resulting nlink==1 entry produces a tombstone ref.
	// Build: file "f.txt" -> /obj (object), with an existing
	// /obj already in place, then run mv. The resulting call
	// to InsertTombstones is what we want to fail.
	ctx := context.Background()
	svc, nodeSvc, buf := newLoggedServiceAndNode(t)

	rootID, err := svc.GetRootNodeID(ctx, "d1")
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

	line := findLine(t, buf.String(), "vfs.mv.tombstone_failed")
	require.NotNil(t, line, "expected vfs.mv.tombstone_failed line in %s", buf.String())
	assert.Equal(t, "ERROR", line["level"])
}

func TestVFSRmLogsCompleted(t *testing.T) {
	ctx := context.Background()
	svc, nodeSvc, buf := newLoggedServiceAndNode(t)

	rootID, err := svc.GetRootNodeID(ctx, "d1")
	require.NoError(t, err)
	root, err := nodeSvc.GetByID(ctx, rootID)
	require.NoError(t, err)

	// Empty file, not a directory: rm without -r succeeds.
	f, err := nodeSvc.CreateFile(ctx, "")
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(ctx, root, "f.txt", f))

	require.NoError(t, svc.Rm(ctx, "d1", []string{"/f.txt"}, false))
	require.NotNil(t, findLine(t, buf.String(), "vfs.rm.completed"),
		"expected vfs.rm.completed line in %s", buf.String())
}

func TestVFSRmLogsErrorOnTombstoneFailure(t *testing.T) {
	ctx := context.Background()
	svc, nodeSvc, buf := newLoggedServiceAndNode(t)

	rootID, err := svc.GetRootNodeID(ctx, "d1")
	require.NoError(t, err)
	root, err := nodeSvc.GetByID(ctx, rootID)
	require.NoError(t, err)

	obj, err := nodeSvc.CreateObject(ctx, node.ObjectContent{Bucket: "b", Key: "k"}, 4)
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(ctx, root, "obj", obj))

	svc.GarbageRecorder = &fakeGC{err: errors.New("kafka down")}
	require.Error(t, svc.Rm(ctx, "d1", []string{"/obj"}, false))

	line := findLine(t, buf.String(), "vfs.rm.tombstone_failed")
	require.NotNil(t, line, "expected vfs.rm.tombstone_failed line in %s", buf.String())
	assert.Equal(t, "ERROR", line["level"])
}

func TestVFSMountLogsCreated(t *testing.T) {
	ctx := context.Background()
	svc, buf := newLoggedService(t)
	require.NoError(t, svc.Mount(ctx, "d1", "/sub", "d2"))
	require.NotNil(t, findLine(t, buf.String(), "vfs.mount.created"),
		"expected vfs.mount.created line in %s", buf.String())
}

func TestVFSUnmountLogsCompleted(t *testing.T) {
	ctx := context.Background()
	svc, nodeSvc, buf := newLoggedServiceAndNode(t)

	rootID, err := svc.GetRootNodeID(ctx, "d1")
	require.NoError(t, err)
	root, err := nodeSvc.GetByID(ctx, rootID)
	require.NoError(t, err)

	mount, err := nodeSvc.CreateMount(ctx, "d2")
	require.NoError(t, err)
	require.NoError(t, nodeSvc.Link(ctx, root, "m", mount))

	require.NoError(t, svc.Unmount(ctx, "d1", "/m"))
	line := findLine(t, buf.String(), "vfs.unmount.completed")
	require.NotNil(t, line, "expected vfs.unmount.completed line in %s", buf.String())
	assert.Equal(t, "d2", line["source_drive"])
}

type fakeGC struct {
	err error
}

func (g *fakeGC) RecordGarbage(_ context.Context, _ []GarbageRef) error {
	return g.err
}
