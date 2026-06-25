# Architecture

> **Philosophy**: Simple, clear responsibility per layer. Following Go standards.

## Design Principles

1. **Explicit over implicit**. Dependency injection is manual only. No DI framework in production.
2. **Interfaces are defined by consumers**. Small, focused interfaces. Not by implementers.
3. **Single responsibility per layer**. Each package does one thing.
4. **Standard library patterns**. `context.Context`, `error` with `%w` wrapping, `log/slog`.
5. **Code generation for boilerplate**. `ent`, `ogen`, `mockery` handle repetitive code. Domain logic stays human-written.

## Layered Architecture

```
cmd/mdrive/                  — Entry point (thin, delegates to internal/cli)

internal/
  core/                      — Domain layer: types + services + repository pattern
    node/                    — POSIX-like inode model (the only persistent entity)
    drive/                   — Drive domain (config + storage + ownership)
    user/                    — User domain (provider identity)

  vfs/                       — Service layer: POSIX filesystem ops (inode tree)
                                path resolution, link/unlink, mv, rm, mount, symlink
                                No S3 I/O. No HTTP. No permission checks.

  upload/                    — Service layer: S3 object lifecycle
                                presigned URLs, multipart completion, gc garbage drain
    s3/                      — AWS SDK v2 client implementation (only consumer)

  permission/                — OpenFGA checker (cross-cutting)

  auth/                      — OIDC + sessions
    session/                 — session.Store + memory/valkey backends

  app/                       — Composition root (HTTP transport, GC jobs)
    app.go                   — builder functions: newInfra, newCrypto, newRepositories, newPerm, newAuth
    apiserver/               — HTTP transport (handlers, middleware, error mapping, security)
      security.go            — AnonSecurity (dev) + auth.SecurityHandler (prod)
    gc/                      — Background cleanup jobs
      store.go               — GarbageRecorder (implements vfs.GarbageRecorder)
      runner.go              — four Runners, each takes only its own deps

  cli/                       — cobra commands (api-server, gc, migrate)
  config/                    — Viper-based configuration loading
  crypto/                    — AES-GCM at-rest cipher for drive storage secrets

pkg/api/                     — Generated ogen code (HTTP types)
ent/                         — Generated ent code (DB ORM)
test/                        — E2E, integration, kind tests
```

Each layer depends **only downward**. The domain (`core/`) has no
external dependencies. The service layers (`vfs/`, `upload/`) use the
domain. The transport (`app/apiserver/`) uses the service layers and
the cross-cutting concerns (`permission/`, `auth/`).

## Where each concern lives

| Concern | Owner |
|---|---|
| Inode tree manipulation | `vfs` |
| Cross-drive path resolution | `vfs` |
| Mount traversal | `vfs` |
| S3 presigned URLs | `upload` |
| S3 object delete | `upload` |
| S3 client implementation | `upload/s3` |
| Tombstone (gc garbage) recording | `app/gc.GarbageRecorder` |
| Tombstone drain | `app/gc.TombstoneCleaner` |
| Permission check (single source) | `handler.requirePerm` |
| OIDC session | `auth` + `auth/session` |
| HTTP transport | `app/apiserver` |
| Background jobs | `app/gc` |
| Composition | `app.New` (chain of small builders) |

## Dependency Injection

Production uses **manual explicit wiring** only:

```go
// internal/app/app.go
func New(ctx context.Context, cfg *config.Config) (*App, error) {
    log := newLogger(cfg.App.Env, cfg.App.LogLevel)
    db, entClient, err := newInfra(ctx, cfg, log)
    cipher, err := newCrypto(ctx, cfg, log)
    repos := newRepositories(entClient, cipher)
    // ...
    return &App{...}, nil
}
```

Each builder is 10-20 lines, single-purpose, fail-fast on its own
concern. The reader can scan `New` top-to-bottom to know what depends
on what; each line is one composition step.

## Interface Pattern

```go
// internal/vfs/service.go — defined by the consumer
type NodeClient interface {
    GetByID(ctx context.Context, id uuid.UUID) (*node.Node, error)
    Link(ctx context.Context, parent *node.Node, name string, child *node.Node) error
    // ...
}

// internal/core/node/service.go — concrete implementation
// node.Service implements NodeClient (verified by _ NodeClient = (*node.Service)(nil))
```

Interfaces are minimal and consumer-defined. A test fake only
implements the methods the consumer actually calls.

## Domain Model

### Inode

POSIX-like metadata (no filename — filename lives in the parent
directory). Fields: `type`, `size`, `nlink`, `mode`, `uid`, `gid`,
`atime`, `mtime`, `ctime`, `crtime`, `flags`, `rev`.

Content types:
- `FileContent` — inline bytes (max 4 KiB)
- `DirContent` — directory entries
- `SymlinkContent` — symlink target (path string)
- `ObjectContent` — reference to an S3 object (bucket + key)
- `MountContent` — pointer to another drive's root

### Drive

A drive is a logical namespace rooted at one inode. Has a
storage config (bucket + region + credentials, encrypted at rest),
an owner, and a soft-delete timestamp.

### User

A user is an external identity (Google OIDC). The local row is
upserted from the OIDC claim on first login.

## Error Handling

Each package owns its own sentinel errors (`ErrXxx`). The
`app/apiserver/error.go` mapper centralises the error→HTTP-code
mapping via `errors.Is` checks for every known sentinel. New
sentinels added to a package must be added to the mapper too.

## Code Generation

| Generator | Schema | Command | Output |
|-----------|--------|---------|--------|
| `ent` | `ent/schema/` | `make gen-ent` | `ent/client.go`, `ent/<entity>.go` |
| `ogen` | `api/openapi.yaml` | `make gen-api` | `pkg/api/oas_*.go` |
| `mockery` | `.mockery.yaml` | `make gen-mock` | `mocks/mock_<interface>.go` |

## Configuration

Viper-based hierarchy: **defaults → config file (YAML) → environment variables**.

Environment variables use `strings.NewReplacer(".", "_")`:
- `database.password` → `DATABASE_PASSWORD`
- `crypto.master_key` → `CRYPTO_MASTER_KEY`

### Fail-fast in production

Two keys are required in production:
- `crypto.master_key` — drives use this for at-rest encryption.
- `openfga.api_url` — every handler runs `requirePerm`; without
  OpenFGA the check is permissive and any user has full access.

In development both are optional. The startup log warns loudly when
either is missing.

## Database

- **ORM**: entgo.io/ent
- **Driver**: lib/pq with otelsql
- **Migrations**: Atlas (versioned in production, auto in dev)
- **Connection pool**: MaxOpenConns: 25, MaxIdleConns: 5

## Authentication

- **OIDC**: Google with PKCE flow (single provider configured;
  the field is reserved for future multi-provider)
- **Session**: Valkey with configurable TTL (memory in dev)
- **Cookie**: `mdrive_session` (HttpOnly, Secure configurable, SameSite=Lax)
- **Context**: `user_id` and `session_id` injected via `auth.SessionFromContext`

## Storage

AWS SDK v2 client (`internal/upload/s3`). Supports AWS S3 and
MinIO via `endpoint` configuration. The interface used by the
service layers is small and consumer-defined:

```go
// internal/upload/service.go
type ObjectStore interface {
    GetPresignedUploadURL(...)
    GetPresignedDownloadURL(...)
    ObjectExists(...)
    DeleteObject(...)
}
```

Presigned URL expiry is fixed per upload (the handler passes the
configured TTL).

## Helm Chart

The chart uses a single `gc-cronjobs.yaml` template that renders
one CronJob per entry in `.Values.gc.jobs` (a `range` over the
map). Adding a fifth GC job is now a single values entry plus an
`internal/app/gc/runner.go` constructor.
## See also

- [DEVELOPMENT.md](DEVELOPMENT.md) — Makefile targets, conventions, build commands
- [TESTING.md](TESTING.md) — Test pyramid, CI integration, build tags
- [ROADMAP.md](ROADMAP.md) — Completed work and upcoming priorities
