# Testing

## Test Pyramid

| Layer | Command | Build Tag | External Deps |
|-------|---------|-----------|---------------|
| Unit | `make test` | (none) | None |
| Integration | `make test-integration` | `//go:build integration` | Postgres + MinIO (testcontainers) |
| E2E | `make test-e2e` | (none) | Postgres + MinIO + Valkey |
| Kind | `make test-kind` | `//go:build kind` | kind cluster + Docker |

## Unit Tests

- Fast, no external dependencies
- Mocks via `mockery` in `mocks/` subdirectories
- Excluded from `make test`: `mocks/`, `test/e2e/`, `test/integration/`, `test/kind/`

```bash
make test
```

## Integration Tests

```bash
make test-integration
```

Shared containers (started once via `sync.Once`):
- PostgreSQL
- MinIO
- Valkey (optionally)

Each test gets a unique database and bucket.

Build tag required:
```go
//go:build integration
package integration
```

## E2E Tests

```bash
make test-e2e
```

Spins up the full HTTP server:
```go
app := serve.NewApp(cfgFile, port, openAPISpec)
```

Tests real HTTP endpoints with session cookies.

## Kind Tests

```bash
make test-kind
```

```go
//go:build kind
package kind
```

1. Builds Docker image
2. Loads into kind cluster
3. Installs Helm chart
4. Verifies deployment, pods, migration job

## Coverage

| Test Type | Coverage File |
|-----------|---------------|
| Unit | `cover-unit.out` |
| Integration | `cover-integration.out` |
| E2E | `cover-e2e.out` |

```bash
make test-all   # all tests with single coverprofile
```

## Testcontainers

Integration suite uses `testcontainers-go`:

```go
sharedPg := postgres.Run(ctx, "postgres:17")
sharedMinio := minio.Run(ctx, "minio/minio")
```

Shared containers are reused across all tests. Per-test database:

```go
s.dbName = "mdrive_test_" + uuid.New().String()
adminDB.Exec("CREATE DATABASE " + s.dbName)
```

## CI Integration

Tests run in CI pipeline:

```
unit-test → integration-test → e2e-test
    ↓
docker-build → kind-test
```

Each test job uploads coverage to codecov.
