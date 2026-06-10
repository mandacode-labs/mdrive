# Development Guide

> Quick start for new contributors.

## Prerequisites

- Go 1.26.1+
- Node.js 24+ (for OpenAPI bundling)
- Docker (for integration tests)
- kind, kubectl, helm (for kind tests)

## Quick Start

```bash
# Install lefthook hooks
make install-hooks

# Build
make build

# Run
make run

# Or manually
./bin/mdrive serve --config config.yaml
```

## Code Generation

```bash
# Generate all
make generate

# Or individually
make gen-ent     # ent schema
make gen-api     # ogen from OpenAPI
make gen-mock    # mockery
```

## Common Commands

```bash
make fmt            # formatting
make vet            # go vet
make lint           # golangci-lint
make test           # unit tests
make test-integration
make test-e2e
make test-kind
make migrate-diff name=add_users
make migrate-apply
make build
make clean
```

## Project Structure

```
cmd/mdrive/              — Entry point
internal/
  application/            — Use cases
    vfs/                  — Virtual filesystem
    storage/              — Storage orchestration
    gc/                   — Garbage collection
  core/                   — Domain entities
    inode/                — POSIX inode
    object/               — S3 object tracking
    user/                 — User/group
  handler/                — HTTP handlers
  service/sysinit/        — System initialization
  auth/                   — OIDC
  session/                — Session management
  system/                 — System management
  user/                   — External user
  config/                 — Configuration
  errors/                 — Domain errors
  middleware/             — HTTP middleware
  telemetry/              — OpenTelemetry
  utils/                  — Context helpers
pkg/api/                  — Generated ogen code
ent/                      — Generated ent code
test/
  e2e/                    — E2E tests
  integration/            — Integration tests
  kind/                   — Kind cluster tests
```

## Conventions

### Package Naming

- `core/<entity>/` — domain entity + interface
- `core/<entity>/repository/` — implementation
- `mocks/` — auto-generated (co-located with interface)

### Interface Definition

Define interfaces **where they are consumed**, not where they are implemented:

```go
// core/inode/repository.go
package inode

type Repository interface { ... }

// repository/ent_inode.go
package repository
func NewRepository(client *ent.Client) inode.Repository { ... }
```

### Error Handling

Use domain errors, not `errors.New`:

```go
// Good
return errors.NotFound("inode not found")

// Bad
return errors.New("inode not found")

// Wrap standard errors
return errors.WrapInternal(err, "failed to create inode")
```

### Context

Always pass `context.Context` as the first parameter. Use context helpers:

```go
userID := utils.GetUserID(ctx)
ctx = utils.ContextWithUserID(ctx, userID)
```

### Configuration

Add new config fields in three places:
1. `internal/config/config.go` — struct definition
2. `setDefaults()` in `config.go` — default value
3. `config.yaml.example` — example

## Testing

See [TESTING.md](TESTING.md).

## Git Workflow

- Feature branches from `main`
- PR template checklist: `make fmt`, `make vet`, `make test`
- All CI checks must pass

## License

IT
