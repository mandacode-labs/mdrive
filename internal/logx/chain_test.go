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

func TestErrorChain(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantInErr  string
		wantKind   string
		wantStatus int
	}{
		{
			name:      "plain cause wrapped",
			err:       errorx.Wrap(errors.New("connection refused"), "auth: token exchange failed"),
			wantInErr: "auth: token exchange failed: connection refused",
			wantKind:  "unknown",
		},
		{
			name:       "sentinel kind inherited",
			err:        errorx.Wrap(errorx.New(errorx.KindNotFound, "vfs: not found"), "vfs: resolve /a/b/c"),
			wantInErr:  "vfs: resolve /a/b/c: vfs: not found",
			wantKind:   "not_found",
			wantStatus: 404,
		},
		{
			name:       "kind override on plain cause",
			err:        errorx.Wrap(errors.New("disk full"), "drive: write failed", errorx.KindUnavailable),
			wantInErr:  "drive: write failed: disk full",
			wantKind:   "unavailable",
			wantStatus: 503,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
			t.Cleanup(func() { slog.SetDefault(prev) })

			Error(context.Background(), tc.err, tc.name)

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("invalid json: %v\n%s", err, buf.String())
			}
			gotErr, _ := entry["err"].(string)
			if !strings.Contains(gotErr, tc.wantInErr) {
				t.Fatalf("err field %q must contain %q", gotErr, tc.wantInErr)
			}
			if got := entry["kind"]; got != tc.wantKind {
				t.Fatalf("kind = %v, want %q", got, tc.wantKind)
			}
			if tc.wantStatus != 0 {
				if got, _ := entry["status"].(float64); int(got) != tc.wantStatus {
					t.Fatalf("status = %v, want %d", got, tc.wantStatus)
				}
			}
		})
	}
}
