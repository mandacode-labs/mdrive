# Testing

## Test Pyramid

| Layer | Command | Excluded by | External Deps |
|-------|---------|-------------|---------------|
| Unit | `make test` | glob: `mocks/`, `test/e2e/`, `test/integration/` | None |
| Integration | `make test-integration` | glob: `mocks/`, `test/e2e/`, `test/integration/` | Stub fakes (no Docker) |
| Ent Integration | `make test-integration-ent` | build tag: `//go:build integration_ent` | Postgres (testcontainers) |
| E2E | `make test-e2e` | glob: `mocks/`, `test/e2e/`, `test/integration/` | Postgres + Valkey (testcontainers) |

Exclusion is by **directory glob** (Makefile `grep -v` patterns) for everything except the Ent Integration suite, which uses the **`integration_ent` build tag** because it is gated by Docker availability.

The `//go:build integration` tag referenced in earlier versions of this doc does not exist on any test file. Removal of the stub-fakes path that previously carried it coincided with the cleanup arc.

## Unit Tests

```bash
make test
```

- Fast, no external dependencies
- Test fakes are hand-written per package (`fakeRepo`, `stubDrive`, `fakeStore`, `userRepoFake`, etc.); generated mockery mocks are not used
- No mocks to maintain: see [PR-43](https://github.com/mandacode-labs/mdrive/pull/43) which removed the generated tree

## Integration Tests (handler-level)

```bash
make test-integration
```

- Spins up `app.App` against stub fakes for vfs/drive/upload/user
- No Docker required — runs anywhere `go test` runs
- Tests the handler/auth/permission/apiserver integration end-to-end without DB

## Ent Integration Tests (real Postgres)

```bash
make test-integration-ent
```

- Per-test Postgres container via `testcontainers-go` (`postgres.Run`)
- Each test creates a fresh schema; `sync.Once` is not used (one container per test for isolation)
- Build tag required:
  ```go
  //go:build integration_ent
  package ent
  ```

## E2E Tests

```bash
make test-e2e
```

- Per-test Postgres + Valkey containers via testcontainers
- Spins up the full HTTP server (`app.New` + `apiserver.NewServer`)
- Tests real HTTP endpoints with session cookies
- Skippable locally: `go test -short ./test/e2e/...` (though no Makefile target sets `-short`)

## Coverage

| Test Type | Coverage File |
|-----------|---------------|
| Unit | (none — `make test` does not produce coverage; CI uploads only when `-coverprofile` is passed) |
| Integration | (none) |
| Ent Integration | `cover-integration-ent.out` |
| E2E | `cover-e2e.out` |

## See also

- [DEVELOPMENT.md](DEVELOPMENT.md) — Makefile targets, conventions, build commands
- [ARCHITECTURE.md](ARCHITECTURE.md) — Layer boundaries and dependency rules