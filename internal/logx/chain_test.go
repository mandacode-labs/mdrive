package logx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

func TestErrorChainRendersOuterInner(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	cause := errors.New("connection refused")
	wrapped := errorx.Wrap(cause, "auth: token exchange failed")
	Error(context.Background(), wrapped, "auth.callback.failed")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, buf.String())
	}
	got, _ := entry["err"].(string)
	want := "auth: token exchange failed: connection refused"
	if !strings.Contains(got, want) {
		t.Fatalf("err field %q must contain %q", got, want)
	}
	kind, _ := entry["kind"].(string)
	if kind != "unknown" {
		t.Fatalf("kind = %q, want unknown", kind)
	}
}

func TestErrorChainWithSentinelKind(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	leaf := errorx.New(errorx.KindNotFound, "vfs: not found")
	outer := errorx.Wrap(leaf, "vfs: resolve /a/b/c")
	Error(context.Background(), outer, "vfs.resolve.failed")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, buf.String())
	}
	got, _ := entry["err"].(string)
	want := "vfs: resolve /a/b/c: vfs: not found"
	if !strings.Contains(got, want) {
		t.Fatalf("err field %q must contain %q", got, want)
	}
	kind, _ := entry["kind"].(string)
	if kind != "not_found" {
		t.Fatalf("kind = %q, want not_found", kind)
	}
	status, _ := entry["status"].(float64)
	if int(status) != 404 {
		t.Fatalf("status = %v, want 404", status)
	}
}

func TestErrorChainWithKindOverride(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	plain := errors.New("disk full")
	degraded := errorx.Wrap(plain, "drive: write failed", errorx.KindServiceDegraded)
	Error(context.Background(), degraded, "drive.write.failed")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, buf.String())
	}
	got, _ := entry["err"].(string)
	want := "drive: write failed: disk full"
	if !strings.Contains(got, want) {
		t.Fatalf("err field %q must contain %q", got, want)
	}
	kind, _ := entry["kind"].(string)
	if kind != "service_degraded" {
		t.Fatalf("kind = %q, want service_degraded", kind)
	}
	status, _ := entry["status"].(float64)
	if int(status) != 503 {
		t.Fatalf("status = %v, want 503", status)
	}
}
