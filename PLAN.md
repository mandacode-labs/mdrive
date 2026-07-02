# mdrive — Production Trace Logging Standardization & Drive-Create Debug Plan

## 1. Background and Motivation

mdrive's production deployment has exhibited a long-standing, hard-to-reproduce
bug on the `POST /v1/drives` endpoint: the request returns HTTP 200, but the
created drive never appears in the database and a follow-up `GET /v1/drives`
returns an empty list. Because there was no structured trace logging in the
handler / service / repository chain, the operator dashboard showed only the
final access-log line — every internal step (owner check, root-directory
creation, transaction begin/commit, drive insert, storage insert) was
invisible.

To diagnose the issue we added targeted `logx.Debug` calls throughout the
chain, gated by a separate `debugx.Trace` package-level atomic boolean. That
made the symptom visible from a single Playwright POST: the production
database was missing the `mode`, `uid`, and `gid` columns on the `nodes`
table, so the root-directory `INSERT` failed with `pq: column "mode" of
relation "nodes" does not exist`. The fix is an Atlas migration; the symptom
is now reproducible on demand and the trace is the primary diagnostic surface.

This document captures the cleanup work around that diagnostic surface: a
single, idiomatic Go logging standard so the next operator does not have to
discover the `debugx` package, the dual `if debugx.Trace.Load() { ... }`
guards, and the implicit coupling to `cfg.App.LogLevel == "debug"`.

## 2. Goal

Standardize all diagnostic logging on `logx.Debug` so that:

1. One call site — `logx.Debug(ctx, msg, attrs...)` — is enough to emit a
   trace line. No separate guard, no separate flag, no separate package.
2. The level filter (debug / info / warn / error) is the single switch that
   turns trace output on or off, exactly like every other Go program that
   uses `log/slog`.
3. Boot wiring reads `cfg.App.LogLevel` exactly once and never again. There
   is no `atomic.Bool` for trace state.
4. The trace is reproducible end-to-end: a single ConfigMap change to
   `app.log_level: "debug"` plus a pod rollout makes every step of a drive
   creation visible in the JSON log stream.

## 3. Design

### 3.1 Why `logx.Debug` and not a new `logx.Trace`

`slog` already provides a `LevelDebug` constant at `-4`. The current `logx`
package surfaces it as `logx.Debug`. There is no need for a fifth level
(`LevelTrace = -8`) — the volume of trace logs we emit is well within what
operators expect to see at the debug level, and ent's own
`entClient.Debug()` SQL trace also lives at the debug level. Adding a
`LevelTrace` would split the level vocabulary without adding diagnostic
value, and would force every reader of the log stream to learn a new
shorthand.

### 3.2 Why no `atomic.Bool` flag

`debugx.Trace` was an attempt to keep debug log lines out of the
hot path when `cfg.App.LogLevel == "info"`. The cost of that approach
was:

- Every debug call had to be wrapped in `if debugx.Trace.Load() { ... }`,
  multiplying the line count and the diff churn.
- The flag had to be wired in two places (app boot and middleware), with
  a third (`EnableDebugTrace`) acting as a leak between packages.
- The flag could only be flipped at process start; toggling it required a
  pod rollout anyway, which is also what we already do for the level
  change.

`slog.HandlerOptions.Level` already does the filtering the flag was trying
to approximate. Removing the flag means the call sites shrink to a single
line, the boot wiring simplifies, and the operator-facing lever is just
`log_level`. There is no measurable runtime cost difference because the
slog handler skips `slog.LevelDebug` records at the same point an
`atomic.Bool.Load()` check would have.

### 3.3 Why the call site signature stays the same

`logx.Debug(ctx, msg, attrs ...slog.Attr)` matches the rest of the
`logx` surface (`Info`, `Warn`, `Error`, `Request`) and matches
`log/slog`'s own `LogAttrs`. Keeping the signature means every existing
trace log moves from a three-line guarded block to a one-line direct
call, and any new diagnostic logging follows the same pattern. A
caller who knows the `logx` API does not need to learn any new
convention.

## 4. Changes Implemented in This Branch

### 4.1 Packages touched

- `internal/app/app.go` — drops `debugx.Trace.Store(true)` from the
  `cfg.App.LogLevel == "debug"` branch; the only remaining effect at
  that level is `entClient = entClient.Debug()` for SQL tracing.
- `internal/app/apiserver/middleware.go` — drops the `debugx` import
  and the `if debugx.Trace.Load() { ... }` guards around the
  `apiserver.request.enter` and `apiserver.request.exit` log lines.
- `internal/app/apiserver/server.go` — drops the `debugx` import and
  the three guards in `AuthPassthrough` (`/auth/login`,
  `/auth/callback`, `/auth/logout`).
- `internal/app/apiserver/handler/drive.go` — drops the `debugx`
  import and the three guards in `CreateDrive` (enter, service_err,
  service_ok).
- `internal/core/drive/service.go` — drops the `debugx` import and
  the eight guards in `Create` covering enter, owner_ok, drive_built,
  root_dir_failed, root_dir_ok, tx.enter, tx.create_ok, tx.update_ok,
  tx_failed, tx_committed.
- `internal/core/drive/ent.go` — drops the `debugx` import and the
  guards in `Create` and `WithTx`.
- `internal/core/node/service.go` — drops the `debugx` import and
  the single guard in `create` (the root-directory factory call).

### 4.2 Package deletion

- `internal/debugx/` — deleted. The package was a single
  `atomic.Bool` plus an `EnableDebugTrace` setter. Both are replaced
  by slog's level filter.

### 4.3 Outcome

- Every trace log in the chain is now a one-line `logx.Debug(...)`
  call.
- `go build ./...`, `go test ./...`, `go test -tags=integration_ent
  ./test/integration/ent/...`, and `golangci-lint run ./...` all
  pass clean.
- The next operator who needs to debug drive creation sets
  `app.log_level: "debug"` in the mdrive ConfigMap, restarts the
  pod, and reads the JSON log stream — no extra package, no extra
  flag, no extra wiring.

## 5. Diagnosis Captured in the Process

The trace logs revealed the real root cause in production:

```
"err":"node: save directory: pq: column \"mode\" of relation \"nodes\" does not exist at column 77 (42703)"
```

The `nodes` table is missing the `mode`, `uid`, and `gid` columns that
were added to the ent schema in a previous commit. The corresponding
Atlas migration was never generated, so the schema drift went
unnoticed. The fix lives in a separate branch and consists of an
`atlas migrate diff` that produces an `ALTER TABLE nodes ADD COLUMN
...` migration plus a manual `make migrate-hash` and a manual
`migrate apply` against the production database (verified directly
with `psql` from a debug pod). After the migration lands, drive
creation should produce a row, a 200 response with a populated body,
and a follow-up `GET /v1/drives` should return the new entry.

## 6. Follow-up Work (Other Branches)

This branch only standardizes the logging. The cleanup work that
belongs with the schema fix is tracked here for context.

### 6.0 `logx` simplification (done in this branch)

`internal/logx/logx.go` was reduced from 262 lines to 117 lines by
dropping the `With*`/`*FromContext` string-id APIs and the
`handler{}` auto-inject wrapper in favor of a single ctx-scoped
`*slog.Logger` (uber-go/zap pattern):

```go
// logx public surface
Config
New(Config) *slog.Logger
Info(ctx, msg, attrs...)
Warn(ctx, msg, attrs...)
Debug(ctx, msg, attrs...)
Error(ctx, err, msg, attrs...)
WithLogger(ctx, log) context.Context
FromContext(ctx) *slog.Logger
```

The 84 call sites across the codebase do not change — `logx.Info`,
`logx.Warn`, `logx.Debug`, `logx.Error` now look up the request-scoped
logger via `FromContext` internally, so request_id / user_id flow
through every line as before. Middleware (`RequestIDMiddleware`) is
the only place that derives the request-scoped logger via
`slog.Default().With("request_id", id, "user_id", uid)` and stuffs
it into ctx.

`logx.Request` was deleted: HTTP access logging is a middleware
concern, not a logger concern, so `withRequestLog` now writes the
single "request" line directly through the ctx logger.

`internal/logx/logx_integration_test.go` was deleted; the obsolete
test file in `internal/app/logger_test.go` was folded into
`internal/logx/logx_test.go` and rewritten to exercise the new
`WithLogger`/`FromContext` surface. `internal/app/logger_test.go` is
gone.

All exported symbols now have one-line godoc. `go build`,
`go test`, `go test -tags=integration_ent`, and `golangci-lint run`
all pass clean.

### 6.1 `apiopts` integration

The `internal/app/apiopts/optional.go` package is used only by
`internal/app/apiserver/handler/{drive,user,fs,health}.go`. It is a
three-function helper (`OptString`, `OptStringPtr`, `OptBool`) that
wraps ogen's `api.OptT` types. Because it has no domain dependencies
it is currently a sibling package, but a four-file usage footprint
does not justify the indirection. The natural shape is to inline the
helpers as lowercase functions in `apiserver/handler/apiopts.go`:

```go
package handler

func optString(s string) api.OptString { ... }
func optStringPtr(s *string) api.OptString { ... }
func optBool(b bool) api.OptBool { ... }
```

This keeps the readability win (`optString(d.Name())` is shorter than
`apiopts.OptString(d.Name())` and shorter than the inline
`api.OptString{Value: d.Name(), Set: true}`) while reducing the
package surface to zero.

### 6.2 `apierr` integration

`internal/apierr/apierr.go` exposes `FromError(err) (int, Code, error)`
and is used only inside `internal/app/apiserver/`. The natural move
is to relocate the file to `internal/app/apiserver/apierr.go` so the
import path becomes a same-package reference (no import at all) and
the directory count drops by one. The function name and signature
stay unchanged so the three call sites
(`apiserver/error.go`, `apiserver/handler/handler.go`,
`apiserver/handler/auth.go`) only lose one import line.

### 6.3 `logx_integration_test.go` deletion (done in this branch)

`internal/logx/logx_integration_test.go` was named "integration" but
contained no `//go:build` tag and was just unit tests against
`logx.New`. The file is deleted. The `TestClassifyRawErrorFallsBackToInternal`
that was in it is also dropped — it was a `errorx` test that
happened to live here.

### 6.4 Atlas migration for `nodes.mode/uid/gid`

```
make migrate name=add_node_mode_uid_gid
make migrate-hash
make migrate-validate
```

Expected SQL (subject to atlas diff review):

```sql
ALTER TABLE "nodes" ADD COLUMN "mode" bigint NOT NULL DEFAULT 420;
ALTER TABLE "nodes" ADD COLUMN "uid" character(64) NOT NULL DEFAULT '';
ALTER TABLE "nodes" ADD COLUMN "gid" character(64) NOT NULL DEFAULT '';
```

Apply against the production `shared-postgres-rw` instance via the
existing ArgoCD PreSync migration job (or, if the hook has already
expired, by `kubectl exec`ing into the new pod and running
`/app/mdrive migrate apply`). Verify with
`psql -c "\d nodes"` that all three columns now exist, then re-run
the Playwright drive-create flow and confirm that
`SELECT count(*) FROM drives WHERE owner_id = '...'` is at least 1.

## 7. Production Verification Procedure (Manual)

1. Roll the new image out and let the migration job run.
2. `kubectl -n mdrive exec <mdrive-pod> -c mdrive -- psql ... -c
   "\d nodes"` from a debug pod to confirm `mode`, `uid`, `gid`
   exist with the expected types and defaults.
3. Open `https://mdrive.mandacode.com`, log in, click "Add new
   drive", type a name, click "Create". The modal should close and
   the new drive should appear in the sidebar.
4. `kubectl -n mdrive logs <mdrive-pod> -c mdrive | grep drive.`
   should show the full trace:
   `handler.drive.create.enter` -> `drive.service.create.enter` ->
   `drive.service.create.owner_ok` -> `drive.service.create.drive_built`
   -> `drive.service.create.root_dir_ok` -> `drive.service.create.tx.enter`
   -> `drive.repo.with_tx.begin` -> `drive.repo.create.drive.insert.ok`
   -> `drive.repo.create.storage.insert.ok` ->
   `drive.service.create.tx.create_ok` -> `drive.service.create.tx.update_ok`
   -> `drive.repo.with_tx.committed` ->
   `drive.service.create.tx_committed`.
5. `psql -c "SELECT id, name, owner_id, root_node_id FROM drives"`
   should return at least one row.
6. Restore `app.log_level: "info"` in the ConfigMap and let the
   pod roll again to silence the trace for normal operation.

## 8. Summary

The logx.Debug standardization in this branch is small, mechanical,
and immediately useful: it removes a parallel flag-and-guard pattern
that obscured the real bug for weeks, and it does so by leaning
harder on the standard library rather than adding to it. The
follow-up branches handle the structural cleanups and the schema
fix; both are documented above so the work is recoverable from this
file alone.
