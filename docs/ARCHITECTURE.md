# Architecture

> **Philosophy**: Simple but powerful. Following Go standards.

## Design Principles

1. **Explicit over implicit**. Dependency injection is manual only. No DI framework in production.
2. **Interfaces are defined by consumers**. Small, focused interfaces. Not by implementers.
3. **Plain structs**. Domain models are simple structs with getters. No inheritance, no generics complexity.
4. **Standard library patterns**. `context.Context`, `error` with `%w` wrapping, `fmt.Errorf`.
5. **Code generation for boilerplate**. `ent`, `ogen`, and `mockery` handle repetitive code. Domain logic stays human-written.

## Layered Architecture

```
cmd/                 — Entry point (thin, delegates to internal/cmd)
internal/
  application/       — Use cases (VFS, storage orchestration, GC)
  core/              — Domain entities (inode, object, user)
  handler/           — HTTP transport layer (ogen adapters)
  service/           — Cross-cutting services (sysinit)
  auth/              — OIDC authentication
  session/           — Session management
  system/            — System (node) management
  user/              — External user management
  config/            — Configuration loading
  errors/            — Domain error types
  logging/           — Logger
  middleware/          — HTTP middleware
  telemetry/         — OpenTelemetry
  utils/             — Context helpers
pkg/api/             — Generated ogen code
ent/                 — Generated ent code
test/                — E2E, integration, kind tests
```

## Dependency Injection

Production uses **manual explicit wiring** only:

```go
// internal/cmd/serve/app.go
func NewApp(...) *App {
    repo := inoderepo.NewRepository(client)
    svc := inode.NewService(repo)
    handler := handler.NewHandler(svc)
    // ...
}
```

Every dependency is constructed explicitly. Simple, traceable, no magic.

Uber FX code exists in `serve.go` but is **test-only** — not used in production.

## Interface Pattern

```go
// core/inode/repository.go — defined by the consumer
package inode

type Repository interface {
    Create(ctx context.Context, in *Inode) (*Inode, error)
    Get(ctx context.Context, id string) (*Inode, error)
    // ...
}

// repository/ent_inode.go — implemented in a sub-package
package repository

func NewRepository(client *ent.Client) inode.Repository {
    return &entRepository{client: client}
}
```

Interfaces are small. GC only needs 4 methods from ObjectService — it defines its own minimal interface.

## Domain Model

### Inode

POSIX-like metadata (no filename). Contains `mode`, `uid`, `gid`, `size`, `link_count`, `atime`, `mtime`, `ctime`, and a JSON `content` blob.

Content types:
- `DirContent` — directory entries
- `SymlinkContent` — symlink target
- `ObjectContent` — reference to an S3 object ID

### Object

Tracks external storage objects. Has `status` (`pending`/`active`), `checksum`, `idempotency_key`, `bucket`, `storage_key`.

### System

A logical node/cluster that owns inodes and objects. Multi-tenancy boundary.

## Error Handling

Single structured domain error:

```go
type Error struct {
    Code       string
    Message    string
    StatusCode int
    Details    map[string]any
}
```

Constructors: `BadRequest`, `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`, `Internal`, `ServiceUnavailable`.

Wrap functions: `WrapInternal`, `WrapBadRequest`, `WrapNotFound` — wrap standard errors with context.

HTTP mapping: `handler/errors.go` maps `*errors.Error` to `ogen` response types using `errors.As`.

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
- `cache.valkey.password` → `CACHE_VALKEY_PASSWORD`

Sensitive keys are explicitly bound.

## Database

- **ORM**: entgo.io/ent (v0.14.6)
- **Driver**: lib/pq with otelsql
- **Migrations**: Atlas (versioned in production, auto in dev)
- **Connection pool**: MaxOpenConns: 25, MaxIdleConns: 5

## Authentication

- **OIDC**: Keycloak with PKCE flow
- **Session**: Valkey/Redis with configurable TTL
- **Cookie**: `mdrive_session` (HttpOnly, Secure configurable, SameSite=Lax)
- **Context**: `user_id` and `session_id` injected into `context.Context`

## Storage

Abstraction behind `Storage` interface:

```go
type Storage interface {
    PutObject(ctx, bucket, key, reader, size) error
    GetPresignedDownloadURL(ctx, bucket, key, expiry) (string, error)
    GetPresignedUploadURL(ctx, bucket, key, contentType, size, checksum, expiry) (string, error)
    DeleteObject(ctx, bucket, key) error
    ObjectExists(ctx, bucket, key) (bool, error)
    // ...
}
```

Implementation: AWS SDK v2. Supports both AWS S3 and MinIO.

Presigned URL expiry is size-based:
- `<= 10MB` → 15 min
- `<= 100MB` → 1 hour
- `<= 1GB` → 3 hours
- `> 1GB` → 6 hours
