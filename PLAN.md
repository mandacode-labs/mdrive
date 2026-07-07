# fs 서브시스템 리팩토링 Plan v6

근거 분석, 결정사항, 실행 단계 정리.

---

## 1. 발견된 문제

### 1.1 컴파일 에러 (4건)

`Node`의 unexported 필드(`atime`/`mtime`/`ctime`/`btime`)를 `vfs` 패키지에서 직접 대입 — 컴파일 실패.

- `internal/fs/vfs/create.go:25-28`
- `internal/fs/vfs/symlink.go:29-32`
- `internal/fs/vfs/mount.go:28-31`

### 1.2 Stub/Panic (5건)

- `internal/fs/superop/{create,stat,purge}.go` 모두 `panic("unimplemented")`
- `internal/fs/mount.go:47-51` `doUnmount`은 `_ = ctx; return nil`
- `test/e2e/main_test.go:113` `vfs.NewService(vfs.Config{...})` — 존재하지 않는 심볼

### 1.3 Service 표면의 syscall 정합성 문제

`internal/fs/service.go`가 syscall 표면을 자처하면서 비-syscall 메서드 포함:
- `Walk`, `WalkOne` — path layer 헬퍼, syscall 아님
- `Create(..., kind NodeKind)` — `mkdir`/`mknod`/`creat` 통합 (Linux에선 별개 syscall)
- `Read`/`Write`가 `[]byte` 반환 — 의미 없는 raw 출력
- `ReadlinkAt`가 `string(UUID)` 반환 — 의미 없음
- `CreateObject` — S3 lifecycle과 fs lifecycle 혼재
- `Remove(rm -rf)` — syscall 아님

### 1.4 VFS 인터페이스가 path layer와 inode ops 혼재

`internal/fs/vfs.go`의 `VFS` 인터페이스가 `lookup`/`walkOne`/`followMount`/`followSymlink`(path layer)와 `create`/`mkdir`/`symlink`/`link`/`unlink`/`rmdir`/`rename`/`read`/`write`/`readlink`/`getattr`/`iterate`/`mount`(inode ops)를 한 인터페이스에 섞고 있음. `Walk`는 fs.Service에 있는데 syscall 아님 — vfs 소유여야 함.

### 1.5 Dentry 구조가 Linux와 차이

```go
type Dentry struct {
    DriveID ulid.ULID       // ← superblock 참조여야 함
    Parent  *Node           // ← Linux: *Dentry
    Name    string
    Node    *Node
}
```

### 1.6 로직 결함

- `vfs.symlink`의 `nodeOp.Create` + `nodeOp.Symlink` 중복 호출
- `nodeop/link.go`의 `IncNLink()` 후 `repo.Write` 누락
- `nodeop/rename.go`의 dst overwrite 미처리
- `walkOne`의 `..` 미처리
- `removeRecursive`의 mount follow 무한 루프 위험
- `fs/link.go:34`에서 srcDentry의 stat 반환 (새 link stat 반환해야 함)

---

## 2. 핵심 아키텍처 결정

| 결정 | 채택 | 근거 |
|---|---|---|
| **VFS = 단일 인터페이스** | ✅ | PathResolver/VFSInode 분리 안 함. Linux `vfs_*` 통합 표면 |
| **`Walk`는 VFS 소유** | ✅ | syscalls `link_path_walk`/`lookup_one`은 VFS 내부. fs.Service에 노출 안 함 |
| **`Open`/`Close`/`Handle` 도입 안 함** | ✅ | mdrive는 stream I/O 모델 아님 (S3 GET/PUT + 4KB inline 스냅샷) |
| **Service = path 기반, VFS = `*Dentry` 기반** | ✅ | 책임 경계 명확: Service는 외부 인터페이스, VFS는 internal 도메인 |
| **Kind별 Service 메서드 분할** | ✅ | `Read`/`Write` 대신 `ReadFile`/`WriteFile`/`ReadObject` 등. 의미 있는 typed 출력 |
| **Content 타입 직접 노출 (DTO 분리 안 함)** | ✅ | `content.FileContent`/`ObjectContent` 등이 곧 Service 반환 타입 |
| **S3 연동 = Presigner 인터페이스로 분리** | ✅ | fs는 S3 SDK 모름, upload가 구현 주입. 책임 분리 + 원자성 |
| **`Unlink`/`Rmdir` 둘 다 유지, VFS 내부 공통화** | ✅ | HTTP 의미 명확, VFS는 `vfs_unlink`/`vfs_rmdir`와 1:1 |
| **Go consumer-side interface 패턴 유지** | ✅ | `fs.NodeOperation` interface in `fs/node.go`, impl in `nodeop/` |

---

## 3. 패키지 구조 (consumer-side interface)

```
internal/fs/                      ← consumer (interfaces + Service)
  ├── service.go                  (Service interface — syscall 표면)
  ├── syscall_*.go                (path-based doX 함수들)
  ├── perm.go                     (requireView, requireEdit, doPathParent)
  ├── stat.go                     (Stat DTO, NodeToStat)
  ├── flags.go                    (NodeKind, Flags, Revision, FileMode)
  ├── dentry.go                   (Dentry struct)
  ├── node.go                     (Node struct + NodeOperation interface)
  ├── super.go                    (Superblock struct + SuperOperation interface)
  ├── vfs.go                      (VFS interface)
  ├── presigner.go                (Presigner interface — S3 presign 위임)
  ├── content/                    (content types — 노출)
  │   ├── content.go              (NodeKind)
  │   ├── file.go                 (FileContent)
  │   ├── object.go               (ObjectContent)
  │   ├── symlink.go              (SymlinkContent)
  │   ├── mount.go                (MountContent)
  │   └── dir.go                  (DirContent, DirEntry)
  ├── vfs/                        ← provider (VFS 구현)
  │   ├── vfs.go
  │   ├── walk.go
  │   ├── create.go
  │   ├── link.go
  │   ├── symlink.go
  │   ├── unlink.go
  │   ├── rmdir.go
  │   ├── rename.go
  │   ├── readwrite.go
  │   ├── getattr.go
  │   ├── iterate.go
  │   └── mount.go
  ├── nodeop/                     ← provider (NodeOperation 구현)
  │   ├── operation.go
  │   ├── lookup.go
  │   ├── create.go
  │   ├── link.go
  │   ├── unlink.go
  │   ├── rmdir.go
  │   ├── rename.go
  │   ├── symlink.go
  │   └── noderepo.go
  ├── superop/                    ← provider (SuperOperation 구현)
  │   ├── operation.go
  │   ├── create.go
  │   ├── stat.go
  │   ├── purge.go
  │   └── sbrepo.go
  └── permission/
```

---

## 4. Service 인터페이스 (Plan v6 확정)

```go
type Service interface {
    // === File ops (NodeKindFile) ===
    CreateFile(ctx, driveID, path string, content *content.FileContent) (Stat, error)
    ReadFile(ctx, driveID, path string) (*content.FileContent, error)
    WriteFile(ctx, driveID, path string, content *content.FileContent) (Stat, error)
    Truncate(ctx, driveID, path string, size int64) error

    // === Object ops (NodeKindObject) ===
    CreateObject(ctx, driveID, path string, content *content.ObjectContent) (Stat, error)
    ReadObject(ctx, driveID, path string) (*content.ObjectContent, error)
    PresignDownload(ctx, driveID, path string, expiry time.Duration) (string, error)
    PresignUpload(ctx, driveID, path string, contentType string, expiry time.Duration) (PresignInfo, error)

    // === Directory ops (NodeKindDirectory) ===
    Mkdir(ctx, driveID, path string) (Stat, error)
    Getdents(ctx, driveID, path string) (*content.DirContent, error)

    // === Symlink ops ===
    SymlinkAt(ctx, driveID, target, linkPath string) (Stat, error)
    ReadlinkAt(ctx, driveID, path string) (*content.SymlinkContent, error)

    // === Link ops ===
    LinkAt(ctx, driveID, srcPath, linkPath string) (Stat, error)
    Unlink(ctx, driveID, path string) error
    Rmdir(ctx, driveID, path string) error

    // === Tree ops ===
    RenameAt(ctx, driveID, srcPath, dstDriveID, dstPath string) error
    Remove(ctx, driveID string, paths []string, opts RemoveOpts) error

    // === Metadata ops ===
    Stat(ctx, driveID, path string, follow bool) (Stat, error)
    SetTimes(ctx, driveID, path string, atime, mtime time.Time) error
    Chmod(ctx, driveID, path string, mode FileMode) error

    // === Mount ops ===
    BindMount(ctx, driveID, mountPath, sourceDriveID string) error
    Unmount(ctx, driveID, mountPath string) error
}
```

---

## 5. VFS 인터페이스 (Plan v6 확정)

```go
type VFS interface {
    // path resolution
    Walk(ctx, driveID ulid.ULID, path string, follow bool) (*Dentry, error)
    WalkOne(ctx, parent *Dentry, name string) (*Dentry, error)
    FollowMount(ctx, mount *Node) (*Dentry, error)
    FollowSymlink(ctx, cur *Dentry, depth int) (*Dentry, error)

    // inode ops — *Dentry 기반
    Create(ctx, parent *Dentry, child *Node, name string) error
    Mkdir(ctx, parent *Dentry, name string) (*Node, error)
    Symlink(ctx, parent *Dentry, name string, targetID uuid.UUID) (*Node, error)
    Link(ctx, oldDentry *Dentry, parent *Dentry, name string) error
    Unlink(ctx, parent *Dentry, name string) error
    Rmdir(ctx, parent *Dentry, name string) error
    Rename(ctx, oldParent *Dentry, oldName string, newParent *Dentry, newName string) error
    Read(ctx, dentry *Dentry) ([]byte, error)            // raw — internal only
    Write(ctx, dentry *Dentry, data []byte) error        // raw — internal only
    Readlink(ctx, dentry *Dentry) (uuid.UUID, error)
    Getattr(ctx, dentry *Dentry) (Stat, error)
    Iterate(ctx, parent *Dentry) ([]DirEntry, error)
    Mount(ctx, parent *Dentry, name string, sourceDriveID ulid.ULID) error
}
```

---

## 6. Dentry 구조 (Plan v6 확정)

```go
type Dentry struct {
    Superblock *Superblock   // DriveID 대신
    Parent     *Dentry        // *Node → *Dentry
    Name       string
    Node       *Node
}

func (d *Dentry) IsRoot() bool { return d.Parent == nil }

func NewRootDentry(sb *Superblock, root *Node) *Dentry
func NewChildDentry(parent *Dentry, name string, node *Node) *Dentry
func NewMountRootDentry(sb *Superblock, mountPoint *Dentry, root *Node) *Dentry
```

---

## 7. Content 패키지 (Plan v6 확정)

```go
type FileContent struct {
    Raw      string `json:"raw"`
    Mime     string `json:"mime,omitempty"`
    Encoding string `json:"enc,omitempty"`
    Checksum string `json:"sum,omitempty"`
}

type ObjectContent struct {
    Bucket   string `json:"bucket"`
    Key      string `json:"key"`
    Mime     string `json:"mime"`
    Checksum string `json:"sum,omitempty"`
    Size     int64  `json:"size"`
}

type SymlinkContent struct {
    TargetID uuid.UUID `json:"target"`
}

type MountContent struct {
    SourceDriveID string `json:"src"`
}

type DirContent struct {
    Entries []DirEntry `json:"entries"`  // "items" → "entries"
}

type DirEntry struct {
    NodeID uuid.UUID `json:"id"`
    Name   string    `json:"name"`
    Kind   NodeKind  `json:"kind"`
}
```

---

## 8. 작업 Phase

### Phase 0 — 컴파일 에러 + 스텁 제거 (빌드 복구)
- [ ] Node.SetTimes 추가
- [ ] vfs/create.go, symlink.go, mount.go의 직접 필드 대입 → SetTimes 호출
- [ ] superop/{create,stat,purge}.go panic 제거 → repo 위임
- [ ] fs/mount.go doUnmount stub 처리
- [ ] test/e2e/main_test.go vfs.NewService → fs.New
- [ ] 빌드 통과 검증

### Phase 1 — Dentry 구조 변경
- Dentry.DriveID 제거, Superblock *Superblock 추가
- Parent *Node → *Dentry
- 영향 10곳 수정 (nodeop/*.go, vfs/lookup.go, vfs/symlink.go, vfs/mount.go)

### Phase 2 — Content 패키지 정리
- DirContent.Items → Entries
- FileContent에 Mime/Encoding/Checksum 필드 추가
- ObjectContent에 Size 필드

### Phase 3 — Service 표면 정리 (kind별 분할)
- fs.Service에서 Walk/WalkOne/Create(통합)/Read/Write 삭제
- kind별 메서드 추가 (Plan v6 확정안)
- vfs.Walk로 진입 통일

### Phase 4 — VFS 인터페이스 정리
- VFS에 Walk/WalkOne 명시
- VFS unlink/rmdir 내부 remove() helper 공통화

### Phase 5 — Presigner 인터페이스 도입
- internal/fs/presigner.go 생성
- fs.Config에 Presigner 필드
- internal/upload/presigner.go s3Presigner 구현
- app.go wiring

### Phase 6 — 로직 결함 수정
- vfs.symlink 중복 호출 정리
- nodeop/link.go IncNLink 후 repo.Write
- nodeop/rename.go dst overwrite
- walkOne .. 처리
- removeRecursive mount 루프
- fs/link.go 새 link stat 반환

### (보류) Phase 7 — Mount 자료구조 재검토
- NodeKindMount → 별도 Mount 구조체 분리
- NodeKindObject → File + xattr 통합
- Dentry cache 도입
- 추가 syscall (access/chown/xattr)

---

## 9. 보류 항목 (사용자 우선순위 확인 후)

- NodeKindMount를 별도 Mount 구조체로 분리
- NodeKindObject를 File + xattr로 통합
- Dentry cache (per-superblock hash table)
- 추가 syscall (access/chown/xattr/statfs)
- Open/Close handle 도입