# Roadmap

## Completed

- [x] POSIX-like inode system with mode, uid, gid, timestamps
- [x] Atomic upload (presigned URL → S3 → transaction → inode link)
- [x] OIDC authentication (Keycloak + PKCE)
- [x] Session management (Valkey/Redis)
- [x] Multi-tenancy (system isolation)
- [x] Garbage collection (pending expiry + orphan detection)
- [x] OpenTelemetry tracing
- [x] Versioned database migrations (Atlas)
- [x] Helm deployment with HPA, CronJob, migration job
- [x] Full test pyramid (unit → integration → e2e → kind)

## Short Term (2026 H1)

- [ ] S3 multipart upload support
- [ ] Directory batch operations (mkdir -p, rm -rf)
- [ ] File sharing (time-limited presigned URL)
- [ ] Write locking (prevent concurrent writes)
- [ ] WebSocket for real-time directory updates
- [ ] Soft delete (trash bin) with restore

## Medium Term (2026 H2)

- [ ] WebDAV or NFS compatibility layer
- [ ] FUSE mount (local filesystem proxy)
- [ ] Distributed locking (multi-node coordination)
- [ ] Write caching (reduce S3 round-trips)
- [ ] Event system (S3 → webhook → notification)

## Long Term (2027)

- [ ] Global deduplication (content-addressable storage)
- [ ] CDN integration (CloudFront / CloudFlare)
- [ ] Storage tiering (S3 → Glacier → Deep Archive)
- [ ] Federation (cross-cluster sync)
- [ ] Audit logging (immutable event log)
