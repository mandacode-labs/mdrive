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
        C[Auth Middleware<br/>Keycloak OIDC]
        D[FsService]
        E[ObjectService]
        F[StorageService]
        G[UserService]
        H[DentryService]
        I[InodeService]
        J[Atomic Upload<br/>Transaction]
        K[GC Collector]
    end

    subgraph "External Dependencies"
        L[(PostgreSQL)]
        M[(S3 / MinIO)]
        N[(Valkey / Redis)]
        O[Keycloak]
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
    participant OS as ObjectService
    participant S3 as S3/MinIO
    participant FS as FsService
    participant IS as InodeService
    participant DS as DentryService
    participant DB as PostgreSQL

    C->>API: POST /upload (initiate)
    API->>OS: InitiateUpload()
    OS->>DB: Create pending object
    OS->>S3: Generate presigned URL
    OS-->>API: {objectID, uploadURL}
    API-->>C: Upload session

    C->>S3: PUT uploadURL (direct upload)
    S3-->>C: 200 OK

    C->>API: POST /upload/{id}/complete
    API->>FS: AtomicUpload()
    FS->>DB: Begin transaction
    FS->>S3: Verify object exists
    FS->>OS: CompleteUpload()
    OS->>DB: Update status=active
    FS->>IS: Create inode
    FS->>DS: Link dentry
    FS->>DB: Commit transaction
    FS-->>API: inode
    API-->>C: 200 OK
```

## Garbage Collection Flow

```mermaid
sequenceDiagram
    participant GC as GC CronJob
    participant OS as ObjectService
    participant DB as PostgreSQL
    participant S3 as S3/MinIO

    GC->>OS: FindPendingOlderThan(24h)
    OS->>DB: SELECT * WHERE status=pending AND age>24h
    DB-->>OS: Expired pending objects
    loop For each expired object
        GC->>OS: DeleteFromDB(id)
        OS->>DB: DELETE object
    end

    GC->>OS: FindActive()
    OS->>DB: SELECT * WHERE status=active
    DB-->>OS: Active objects
    loop For each active object
        GC->>S3: ObjectExists(bucket, key)
        S3-->>GC: false (orphan)
        GC->>OS: DeleteFromDB(id)
        OS->>DB: DELETE object
    end
```

## Testing

| Test Type | Command | Description |
|-----------|---------|-------------|
| Unit | `make test` | Fast, no external deps |
| Integration | `make test-integration` | Postgres + MinIO via testcontainers |
| E2E | `make test-e2e` | Full HTTP server + database |
| Kind | `make test-kind` | Kind cluster + Helm chart |

## Quick Start

```bash
# Install hooks
make install-hooks

# Build
make build
./bin/mdrive serve --config config.yaml

# Run tests
make test
make test-integration
make test-e2e
make test-kind  # requires kind, kubectl, helm, docker
```

## Documentation

- [Architecture](docs/ARCHITECTURE.md) — Design philosophy, patterns, layers
- [Development](docs/DEVELOPMENT.md) — Quick start, conventions, commands
- [Testing](docs/TESTING.md) — Test pyramid, CI integration
- [Roadmap](docs/ROADMAP.md) — Completed, short term, medium term, long term

## Project Structure

```
.
├── api/                    # OpenAPI specs
├── build/                  # Dockerfiles
├── cmd/                    # Entry points
├── deployment/             # Helm charts
├── ent/                    # Ent ORM schema & generated code
├── internal/
│   ├── application/        # App services (fs, storage, gc)
│   ├── core/              # Domain models (inode, object, user, dentry)
│   ├── handler/           # HTTP handlers
│   └── cmd/serve/         # Server DI wiring
├── pkg/api/               # Generated ogen code
└── test/
    ├── e2e/               # End-to-end tests
    ├── integration/       # Integration tests (testcontainers)
    └── kind/              # Kind cluster tests
```

## License

IT
