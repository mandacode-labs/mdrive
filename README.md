# mdrive

A distributed POSIX-like filesystem backed by object storage (S3/MinIO), built with Go.

## Architecture

```mermaid
graph TB
    subgraph Client
        A[Web Client / CLI]
    end

    subgraph "mdrive Server"
        B[HTTP API / ogen]
        C[auth.SecurityHandler<br/>OIDC session]
        D[vfs.Service]
        E[upload.Service]
        F[permission.Authorizer]
        G[user.Service]
        H[node.Service]
        I[drive.Service]
        J[upload.Service<br/>presign + complete]
        K[gc.Runners]
    end

    subgraph "External Dependencies"
        L[(PostgreSQL)]
        M[(S3 / MinIO)]
        N[(Valkey / Redis)]
        O[OIDC provider]
    end

    A -->|HTTP| B
    B --> C
    C --> D
    C --> E
    C --> F
    D --> I
    D --> H
    D --> J
    E --> M
    F --> D
    F --> E
    G --> L
    I --> L
    H --> I
    J --> L
    J --> M
    K --> E
    K --> M
    C --> O
    B --> N
```

## File Upload Flow

```mermaid
sequenceDiagram
    participant C as Client
    participant API as HTTP API
    participant US as upload.Service
    participant S3 as S3/MinIO
    participant VFS as vfs.Service
    participant NS as node.Service
    participant DB as PostgreSQL

    C->>API: POST /upload (initiate)
    API->>US: InitiateUpload()
    US->>DB: Create upload token
    US->>S3: Generate presigned URL
    US-->>API: {uploadID, uploadURL}
    API-->>C: Upload session

    C->>S3: PUT uploadURL (direct upload)
    S3-->>C: 200 OK

    C->>API: POST /upload/{id}/complete
    API->>US: CompleteUpload()
    US->>S3: Verify object exists
    US->>NS: CreateObject + Link
    US->>DB: Delete token
    US-->>API: inode
    API-->>C: 200 OK
```

## Garbage Collection Flow

```mermaid
sequenceDiagram
    participant GC as gc.Runner
    participant DB as PostgreSQL
    participant S3 as S3/MinIO

    rect rgba(80,80,120,0.1)
        Note over GC: TombstoneCleaner
        GC->>DB: scan tombstones by group
        loop For each tombstone group
            GC->>DB: delete tombstone row
            GC->>S3: delete S3 object
        end
    end

    rect rgba(80,80,120,0.1)
        Note over GC: DrivePurger
        GC->>DB: scan drives soft-deleted > retention
        loop For each drive
            GC->>DB: hard-delete drive row
        end
    end

    rect rgba(80,80,120,0.1)
        Note over GC: UploadExpirer / SessionExpirer
        GC->>DB: scan expired tokens via TokenScanner
        loop For each expired token
            GC->>S3: delete orphaned object (best-effort)
            GC->>DB: delete token row
        end
    end
```

## Testing

| Test Type | Command | Description |
|-----------|---------|-------------|
| Unit | `make test` | Fast, no external deps |
| Integration | `make test-integration` | Handler + stub fakes (no Docker) |
| Integration (Ent) | `make test-integration-ent` | Real Postgres via testcontainers |
| E2E | `make test-e2e` | Full HTTP server + Postgres + Valkey |

## Quick Start

```bash
# Install hooks
make install-hooks

# Build
make build
./bin/mdrive api-server run --config config.yaml

# Run tests
make test
make test-integration-ent  # requires Docker
make test-e2e              # requires Docker
```

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — Design philosophy, patterns, layers
- [Development](docs/DEVELOPMENT.md) — Quick start, conventions, commands
- [Testing](docs/TESTING.md) — Test pyramid, CI integration
- [Roadmap](docs/ROADMAP.md) — Completed, short term, medium term, long term

## Project Structure

```
.
├── api/                    # OpenAPI specs (ogen input)
├── charts/mdrive/          # Helm chart
├── cmd/mdrive/              # Entry point (thin, delegates to internal/cli)
├── config.yaml.example     # Example config (see charts/mdrive/values.yaml for canonical)
├── docs/                   # Architecture, development, testing, roadmap
├── ent/                     # Ent ORM schema & generated code
├── Dockerfile               # Container image build
├── internal/
│   ├── core/                # Domain layer (node, drive, user) — no I/O, no HTTP
│   ├── vfs/                 # Service: POSIX inode-tree manager
│   ├── upload/              # Service: S3 object lifecycle (+ s3/ client)
│   ├── permission/          # OpenFGA Authorizer (cross-cutting)
│   ├── auth/                # OIDC + sessions (cross-cutting)
│   ├── app/                 # Composition root (HTTP transport, GC jobs)
│   ├── apiopts/             # ogen Opt* wrapper helpers
│   ├── cli/                 # cobra commands
│   ├── config/              # Viper config loading
│   └── crypto/              # At-rest cipher for drive secrets
├── pkg/api/                 # Generated ogen code
└── test/
    ├── e2e/                 # E2E tests (Postgres + Valkey, testcontainers)
    └── integration/         # Handler integration tests (stub fakes + ent+testcontainers)
```

See `docs/ARCHITECTURE.md` for the layer-responsibility table.

## License

IT
