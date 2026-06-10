# Roadmap

## Priority 1: System Stabilization

- [ ] Stabilize error handling and reduce edge cases
- [ ] Harden the CI/CD pipeline for reliability
- [ ] Improve test coverage across critical paths
- [ ] Add circuit breakers and graceful degradation for external services
- [ ] Strengthen logging and observability (tracing, metrics, alerting)

## Priority 2: Linux System Compatibility & Flexibility

- [ ] Refactor existing methods to align closer with Linux system call semantics
- [ ] Improve POSIX compliance (mode, uid, gid, permission handling)
- [ ] Add flexibility for various Linux filesystem behaviors
- [ ] Support for hard links, symbolic links, and special files

## Priority 3: Storage Disk (Block Device) Concept

- [ ] Introduce storage disk abstraction — users can configure S3 (or other storage) as a block device
- [ ] Pre-register storage credentials and endpoints (secret management)
- [ ] Template-like behavior: select target disk per upload / file operation
- [ ] Multi-backend support (switch between S3, MinIO, other object storage)

## Priority 4: Permission System & User Management Application

- [ ] Build internal permission system (ACL, RBAC)
- [ ] Add user/group management application
- [ ] Provide self-service user management interface
- [ ] Implement role-based access control for systems and files
