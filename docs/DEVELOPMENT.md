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
make migrate name=add_users   # ent diff + lint + fga model transform
make migrate-lint             # atlas lint only
make migrate-apply            # apply SQL migrations to DB
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
  logging/                structured logging (zerolog)
  permission/             OpenFGA authorization
  storage/                external storage
    s3/                   S3/MinIO client
  upload/                 presigned upload registry (Valkey/Memory)
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
