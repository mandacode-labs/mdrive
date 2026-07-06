# mdrive Linux-style Layering Refactor

## Goal

Reorganize the codebase to mirror the Linux kernel architecture:
`handler → syscall → vfs → operation (nodeop, driveop)`.

Drop the `internal/core/` directory. Move `node` and `drive` into the vfs
layer as `inode_operations` and `drive_operations`. Introduce a new
`syscall` layer that composes vfs primitives into filesystem-shaped
operations the handler can call. Place `upload` as a peer of `syscall`
(presigned URLs are not filesystem operations).

## Constraints

- **Naming follows Linux**: vfs exposes `inode_operations`-level
  primitives only (Mknod, Link, Symlink, Unlink, Rmdir, Rename, Lookup)
  and `drive_operations` (Create, Get, Update, SoftDelete, Restore,
  Purge, List, Initialize, Get, Delete). No filesystem-shaped methods
  (Mkdir, Touch, Rm, Mv, Ls, Cat, Write, ...) on vfs.
- **Data access**: `Repository` naming. `NodeRepository`,
  `DriveRepository`, `StorageRepository`. File names `noderepo.go`,
  `driverepo.go`, `storagerepo.go` (Go convention: lowercase, no
  camelCase filename boundary).
- **Interfaces**:
  - vfs: `NodeOperation`, `DriveOperation` — public interfaces,
    unexported `nodeOperation` / `driveOperation` impls.
  - vfs.NodeOperation impl in `internal/vfs/nodeop/`.
  - vfs.DriveOperation impl in `internal/vfs/driveop/`.
  - permission: `permission.Authorizer` (already in `internal/vfs/permission/`).
- **user**: top-level service in `internal/user/` (already moved).
- **upload**: top-level service in `internal/upload/`, peer of syscall.
- **syscall**: thin wrapper. param normalization, path resolution, fs
  command composition. Lives in `internal/syscall/`.
- **Resolver**: `internal/vfs/resolver.go` (top-level). nameidata-style
  walk. Each component: perm check, nodeop.Lookup, mount crossing,
  symlink follow.
- **Mount crossing**: read-only follow only, single-level.
- **Permission**: Resolver checks per component; handler checks once
  at entry. Redundant by design.
- **Cache**: none. Each resolve hits the DB.
- **Dentry**: internal type for nodeop.Lookup. Resolver returns
  `Resolved` / `ResolvedParent` to callers.
- **unexported impls + `var _ Interface = (*impl)(nil)` in the impl
  file**: confirmed pattern.

## Progress

### Phase 0 — Commit current work ✅ ALREADY COMMITTED
- [x] commit `c0b8859`: core layer를 vfs로 이동 (node, drive, permission)
- [x] commit `625d558`: drivestorage → storage rename
- [x] commit `2ee125c`: ent schema 업데이트
- [x] commit `c650e5b`: crtime → btime rename
- [x] commit `ed77318`: nodeop과 vfs 구조 업데이트
- [x] commit `b99046b`: constructor 이름 변경

Dirty/untracked 정리 필요 (작업 중인 추가 변경):
- `internal/vfs/drive.go` (Root 타입 *uuid.UUID → uuid.UUID)
- `internal/vfs/drive/doc.go` (deprecated 표기)
- `internal/vfs/driveop.go` (인터페이스 변경)
- `internal/vfs/nodeop/{block.go, operation.go}` (error wrap + perm 추가)
- `internal/vfs/driveop/` 전체 신규 (untracked)

### Phase 1 — Rename data-access layer to Repository ✅ DONE
- [x] `internal/vfs/nodeop/block.go` → `noderepo.go`,
      `BlockStorage` → `NodeRepository` (also renamed `blockStorage` →
      `nodeRepo`)
- [x] `internal/vfs/driveop/block.go` → split into `driverepo.go`
      (Drive) and `storagerepo.go` (Storage)
- [x] introduce `StorageRepository` interface
- [x] `var _ NodeRepository = (*nodeRepo)(nil)`,
      `var _ DriveRepository = (*driveRepo)(nil)`,
      `var _ StorageRepository = (*storageRepo)(nil)` in impl files
- [x] `internal/vfs/storage.go`: add `Provider()` getter to vfs.Storage
      so StorageRepository can map ent.provider → vfs.Storage
- [x] file naming: `noderepo.go`, `driverepo.go`, `storagerepo.go`
      (Go convention — all-lowercase, no camelCase filename boundary)
- [x] `SetProvider(entstorage.Provider(s.Provider().String()))`
      in storagerepo.go (use Stringer, not `string(...)` cast)

### Phase 2 — driveop CRUD completion
- [ ] implement `InitializeDrive`, `GetDrive`, `DeleteDrive`
- [ ] add `CreateDrive`, `UpdateDrive`, `SoftDeleteDrive`,
      `RestoreDrive`, `PurgeDrive`
- [ ] add `ListByOwner`, `ListDeletedByOwner`, `ListDeletedForAdmin`
- [ ] wire `RootDirectoryCreator` (creates drive root inode) +
      `OwnerChecker` (user.Repository.Exist) + `crypto.Cipher`
- [ ] cascade delete Storage on Drive purge (ent schema already does
      Cascade on delete)

### Phase 3 — nodeop completion
- [ ] implement `Unlink` (remove entry, decrement nlink, possibly
      destroy inode when nlink==0)
- [ ] implement `Rmdir` (refuse if non-empty, else destroy)
- [ ] harden transaction boundaries on Mknod/Link/Symlink/Rename

### Phase 4 — vfs Resolver (nameidata-style)
- [ ] `internal/vfs/resolver.go`:
      `Resolver` interface + unexported `resolver` impl
      + `Resolved` / `ResolvedParent` structs
- [ ] walk state: current drive, current node, accumulated path,
      remaining components
- [ ] per component: perm check → nodeop.Lookup → mount crossing →
      symlink follow (read path only)
- [ ] mount crossing: transition to source drive root, re-check perm
      on source drive

### Phase 5 — syscall layer
- [ ] `internal/syscall/doc.go`: package doc
- [ ] `internal/syscall/fs.go`: `FS` interface + `fs` impl
      - methods: `Mkdir`, `Touch`, `Rm`, `Mv`, `Ls`, `Cat`, `Write`,
        `WriteLarge`, `Symlink`, `Hardlink`, `Mount`, `Unmount`,
        `Realpath`, `Stat`, `Lstat`, `Readlink`,
        `ResolveForPermission`
      - each: Resolver → perm check → nodeop composition
- [ ] `internal/syscall/upload.go`: thin wrapper over `upload.Service`
- [ ] `internal/syscall/drive.go`: thin wrapper over
      `vfs.DriveOperation`
- [ ] `internal/syscall/user.go`: thin wrapper over `user.Service`
- [ ] `internal/syscall/auth.go`: thin wrapper over `auth.Service`
- [ ] `internal/syscall/wire.go`: constructors (`NewFS`, `NewUpload`,
      `NewDrive`, `NewUser`, `NewAuth`)

### Phase 6 — Handler uses syscall
- [ ] `internal/app/apiserver/handler/handler.go`: fields take
      `syscall.FS`, `syscall.Upload`, `syscall.Drive`, `syscall.User`,
      `syscall.Auth`
- [ ] `handler/fs.go`: `h.fs.Mkdir(...)` → `h.fs.Mkdir(...)`
      (call already on syscall.FS after phase 5)
- [ ] `handler/upload.go`, `handler/drive.go`, `handler/user.go`,
      `handler/auth.go`: switch from `vfs.Service` /
      `core/drive.Service` / `core/user.Service` /
      `upload.Service` → respective `syscall.*`
- [ ] remove `vfs.Service` field, `drive.Service` field, etc.

### Phase 7 — app/server/gc import cleanup
- [ ] `internal/app/app.go`: `core/*` → `internal/user`,
      `internal/vfs/driveop`, `internal/vfs/permission`; fields take
      syscall types where handler-facing
- [ ] `internal/app/apiserver/server.go`: same; `NewServer` args
      become syscall types
- [ ] `internal/app/apiserver/handler/auth_test.go`: `core/user/mocks`
      → `internal/user/mocks`
- [ ] `internal/app/gc/runner.go`: `core/drive` →
      `internal/vfs/driveop` (uses `DriveOperation`); `DrivePurger`
      calls `ListDeletedForAdmin` + `Purge`
- [ ] `internal/app/gc/store.go`: `ent/drivestorage` → `ent/storage`

### Phase 8 — auth import cleanup
- [ ] `internal/auth/auth.go`: `core/user` → `internal/user`
- [ ] `internal/auth/testing.go`: same
- [ ] `internal/auth/flow.go`: same
- [ ] `internal/auth/security_test.go`: same + mocks path

### Phase 9 — upload import cleanup
- [ ] `internal/upload/service.go`: `core/node` (Node, NodeKind,
      ObjectContent, ReadObject) → `internal/vfs`; `core/drive`
      (Service, GetStorage) → `internal/vfs/driveop` or
      `internal/vfs` Drive type
- [ ] `internal/upload/service_test.go`: mocks path updates
- [ ] `internal/upload/mocks/mock_service.go`: imports

### Phase 10 — Delete old drive package
- [ ] remove `internal/vfs/drive/` (deprecated package)
- [ ] remove `internal/vfs/drive/mocks/`
- [ ] verify no remaining imports

### Phase 11 — Regenerate mocks
- [ ] `make gen-mock`
- [ ] impacted: `internal/user/mocks/`, `internal/upload/mocks/`,
      `internal/vfs/permission/mocks/`, `internal/vfs/driveop/mocks/`
      (new), `internal/vfs/nodeop/mocks/` (new),
      `internal/app/gc/mocks/`, `internal/app/apiserver/handler/mocks/`

### Phase 12 — Test updates
- [ ] `test/integration/setup_test.go`: `zeroFS` / `zeroUpload` etc.
      satisfy `syscall.FS` / `syscall.Upload` interfaces; remove
      `core/*` imports
- [ ] `test/e2e/main_test.go`: same; bootstrap uses syscall types
- [ ] `test/e2e/drive_test.go`, `fs_test.go`, `fs_extra_test.go`,
      `migration_test.go`: any core/* references

### Phase 13 — Verify
- [ ] `go build ./...`
- [ ] `go test ./internal/...`
- [ ] `go test ./test/integration/...`
- [ ] `make lint`
- [ ] `gosec ./...`

## Decisions Log

- 2026-07-06: vfs = `NodeOperation + DriveOperation`. handler-스타일
  명령어는 vfs에 두지 않고 syscall이 합성.
- 2026-07-06: `user.Service`는 그대로 top-level.
- 2026-07-06: `upload`는 syscall과 동급 패키지 (presigned URL은 fs op
  아님).
- 2026-07-06: 데이터 저장 계층 = `Repository`. `NodeRepository`,
  `DriveRepository`, `StorageRepository`. 파일명 `noderepo.go`,
  `driverepo.go`, `storagerepo.go`.
- 2026-07-06: Resolver는 `internal/vfs/resolver.go` top-level.
  nameidata-style walk state.
- 2026-07-06: Mount crossing = read-only follow only, single-level.
- 2026-07-06: Permission = Resolver 안 매 component + Handler 진입 시
  중복 체크 (mount crossing 시 source drive 재체크).
- 2026-07-06: Cache 없음. 매 resolve마다 DB hit.
- 2026-07-06: Dentry는 nodeop.Lookup 내부 표현. Resolver는
  `Resolved` / `ResolvedParent` 반환.
- 2026-07-06: Phase 0는 의미별 5 commits.

## Risk Notes

- driveop에 CRUD 6개 신규 메서드 추가 (인터페이스 확장).
- Mount crossing 시 source drive perm 재체크 정확성.
- Root directory 자동 생성 위치 (CreateDrive 또는 별도 op).
- Owner check: driveop이 user.Repository 직접 import (user 패키지
  의존).
- crypto.Cipher (storage secret) driveop에 추가.
- cascade delete Storage on Drive purge (ent schema Cascade).
- e2e (Docker 필요) — verify 단계에서 스킵.