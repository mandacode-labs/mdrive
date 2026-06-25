# Development Guide

## Prerequisites

- Go 1.24+
- Node.js 24+ (for OpenAPI bundling)
- Docker (for integration tests)
- `fga` CLI (for OpenFGA model transform)

## Quick Start

```bash
# Install hooks
make install-hooks

# Build
make build

# Run
make run
```

## Code Generation

```bash
make generate    # all: gen-ent + gen-api + gen-mock + gen-fga
make gen-ent     # ent schema from ent/schema/
make gen-api     # ogen OpenAPI → pkg/api/
make gen-mock    # mockery interfaces → mocks/
make gen-fga     # model.fga → model.json
```

## Migrations

```bash
make migrate name=add_users   # ent diff + atlas validate + fga model transform
make migrate-hash              # regenerate migration hash (after ent schema edits)
make migrate-validate          # atlas validate only
make migrate-apply             # apply SQL migrations to DB
```

## Testing

```bash
make test              # unit tests
make test-integration  # integration tests
make test-e2e          # e2e tests
```

## Project Structure

```
cmd/mdrive/               entry point
api/                      OpenAPI spec
  openapi.yaml            top-level spec
  endpoints/              path definitions (drive, fs, auth, upload, user)
  schemas/                component schemas
pkg/api/                  generated ogen code (package api)
ent/                      generated ent code
internal/
  app/                    application wiring (DI)
    apiserver/            HTTP server (server.go, handler/, error.go)
    gc/                   GC runner
  auth/                   OIDC authentication (Zitadel)
    authenticator.go      auth.Service, token exchange, PKCE
    security.go           ogen SecurityHandler, session middleware
    session/              session.Store, ValkeyStore, MemoryStore
  cli/                    cobra commands (api-server, gc)
  config/                 viper-based configuration
  core/                   domain entities
    drive/                drive.Service, Repository, Storage
    node/                 node.Service, Repository, POSIX types
    user/                 user.Service, Repository
  crypto/                 AES-256-GCM encryption
  permission/             OpenFGA authorization
  upload/                 presigned upload registry (Valkey/Memory)
    s3/                   S3/MinIO client
  vfs/                    virtual filesystem orchestration
```

## Conventions

### Consumer-declared interfaces

Interfaces are defined where they are consumed:

```go
// handler/handler.go — consumer
type FSClient interface { ... }
type AuthClient interface { ... }

// vfs/service.go — consumer of downstream services
type NodeClient interface { ... }
type DriveClient interface { ... }

// vfs.Service satisfies handler.FSClient
var _ handler.FSClient = (*vfs.Service)(nil)
```

### Concrete types = Service / Client suffix

```
drive.Service, node.Service, user.Service, auth.Service
s3.Client
```

### Test files

One `*_test.go` per domain file. Uses `testify/assert` + `testify/require`.

### Configuration

Add new config fields in three places:
1. `internal/config/config.go` — struct field + `mapstructure` tag
2. `setDefaults()` — default value
3. `config.yaml.example` — example with comments

Environment variables: viper `AutomaticEnv` maps `openfga.api_token` → env `OPENFGA_API_TOKEN`.

## OpenFGA Setup

```bash
# One-time store creation
fga store create --name "mdrive"
# → store_id: "01J..." → put in config.yaml
```

`make gen-fga` transforms `internal/permission/model.fga` (DSL) → `model.json` (API format). On startup with `api_url` configured:

- `store_id` is required — server fails without it
- `authorization_model_id` is optional — if empty, writes the embedded model and uses the returned ID

## Zitadel Auth

- `api_url` + `client_id` + `issuer` must be configured
- Google OAuth: `GET /auth/google`, `POST /auth/google/native`
- Sessions stored in Valkey (or memory for dev)
- PKCE enforced via session-backed code verifier storage

## Layer responsibilities

| Layer | Responsibility | Does NOT |
|---|---|---|
| `core/*` | Domain types + service + repository | I/O, HTTP, S3, permissions |
| `vfs` | Inode tree ops (link/unlink/mv/rm/mount/...) | S3 I/O, HTTP, permissions |
| `upload` | S3 lifecycle (presign/complete/delete) | Node tree, permissions |
| `app/apiserver` | HTTP transport + perm gate + error mapping | Domain logic, persistence |
| `app/gc` | Background cleanup jobs | Domain logic |
| `permission` | OpenFGA checker | Storage, sessions |
| `auth` | OIDC + sessions | HTTP, domain |

If you find yourself adding code to a layer that doesn't match its
responsibility, the layer is wrong — open a PR to move the code.

## Local development quick path

```bash
docker run -d --name mdrive-pg -e POSTGRES_PASSWORD=mdrive -p 5432:5432 postgres:17-alpine
docker run -d --name mdrive-valkey -p 6379:6379 valkey/valkey:8-alpine

cp config.yaml.example config.yaml
# Edit config.yaml: set crypto.master_key (or accept dev warning),
# leave openfga empty (dev mode uses AnonSecurity).

make run
# Server at http://localhost:8080
```

## See also

- [ARCHITECTURE.md](ARCHITECTURE.md) — Layer boundaries, dependency rules
- [TESTING.md](TESTING.md) — Test pyramid, build tags, coverage
