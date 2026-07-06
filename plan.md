# mdrive vfs Linux-style Layering Refactor

## Goal

Restructure `internal/vfs` to mirror the Linux kernel VFS layer:

- `vfs.VFS` exposes the high-level vfs_* helpers (Create,
  Symlink, Mount, Unlink, Rmdir, Rename, Link, Stat, Lstat,
  Readlink, Read, Write, WriteObject, IterateDir) plus
  mdrive-specific drive CRUD.
- `vfs.NodeOperation` mirrors Linux `inode_operations`
  (Lookup, Create, Link, Symlink, Unlink, Rmdir, Rename).
- `vfs.DriveOperation` holds drive + storage CRUD (mdrive
  domain).
- Path resolution is internal — each vfs command takes a
  path and resolves internally (perm check + mount crossing +
  symlink follow). No Walk on the public surface.
- `vfs.Dentry` mirrors Linux `struct dentry` (parent + name +
  backing node).

Differences from Linux (deliberate, documented):

1. **No super_operations** — node operations own the full
   transaction (in-memory + DB write).
2. **Drive CRUD is mdrive-specific** — multi-tenant storage
   unit.
3. **Mount crossing**: read-only follow, single-level.
4. **Garbage collection deferred** — no GarbageRef,
   GarbageRecorder in vfs.
5. **Read returns inline data only** — file-kind inline data.
   Object-kind uses upload.PresignDownload (separate flow).
6. **Create creates empty inode** — kind-specific data set
   by Write/WriteObject/Mount/Symlink. Mirrors Linux's
   open(2, O_CREAT) + write(2) split.

## Constraints

- `vfs.VFS` is the high-level dispatcher. It owns the canonical
  fs command surface.
- `vfs.NodeOperation` is the inode-level callback set.
- `vfs.DriveOperation` is drive-level CRUD.
- Path resolution is private to vfs. Caller passes a path;
  vfs runs `resolveTarget` / `resolveParent` internally.
- `vfs.Dentry` is `{Parent *Node, Name string, Node *Node}` —
  Linux struct dentry simplified.
- `vfs.content` is primitive structs only. Does not import
  vfs. Translate via `vfs.ParseNodeKind(string)` /
  `vfs.NodeKindString(NodeKind)`.
- `vfs.ObjectRef` is the public input for WriteObject. vfs
  converts it to its internal content.ObjectContent. Callers
  do not touch internal storage structs.
- nlink == 0 in `nodeop.Unlink` triggers immediate
  `nodeRepo.Destroy`.
- Permission: Resolver/Walk does per-drive checks; high-level
  helpers do entry-level checks (redundant by design).

## vfs package layout

```
internal/vfs/
├── doc.go              # package doc
├── node.go             # Node, NodeKind, Flags, Revision, MaxDataSize, Is* helpers
├── dentry.go           # Dentry
├── drive.go            # Drive model
├── storage.go          # Storage model
├── nodeop.go           # NodeOperation interface (Linux inode_operations)
├── driveop.go          # DriveOperation interface (mdrive-specific)
├── objectref.go        # ObjectRef — public input for WriteObject
├── vfs.go              # VFS interface + unexported vfs struct + path walk + NewVFS
├── create.go           # Create + Symlink
├── mount.go            # Mount
├── remove.go           # Unlink + Rmdir
├── rename.go           # Rename + Link
├── stat.go             # Stat + Lstat + Readlink
├── io.go               # Read + Write + WriteObject
├── iteratedir.go       # IterateDir
├── drivecr.go          # Drive CRUD (pass-through + owner/admin enforcement)
├── perm.go             # isAdmin (admin check helper)
└── nodeop/, driveop/   # impl packages
```

## vfs.VFS surface (22 methods, Walk internal)

```go
// vfs/vfs.go
type VFS interface {
    // Inode creation. Linux vfs_create + vfs_mkdir + vfs_mknod
    // (unified via kind). Empty inode only; data set by kind-specific
    // command below.
    Create(ctx context.Context, driveID string, path string, kind NodeKind) (*Node, error)
    Symlink(ctx context.Context, driveID string, target string, linkPath string) error
    Mount(ctx context.Context, driveID string, mountPath string, sourceDriveID string) error

    // Inode removal. Linux vfs_unlink + vfs_rmdir (separated).
    Unlink(ctx context.Context, driveID string, path string) error
    Rmdir(ctx context.Context, driveID string, path string) error

    // Inode mutation. Linux vfs_rename + vfs_link.
    Rename(ctx context.Context, srcDriveID string, srcPath string, dstDriveID string, dstPath string) error
    Link(ctx context.Context, driveID string, srcPath string, linkPath string) error

    // Info. Linux vfs_stat + vfs_lstat + vfs_readlink.
    Stat(ctx context.Context, driveID string, path string) (*Node, error)
    Lstat(ctx context.Context, driveID string, path string) (*Node, error)
    Readlink(ctx context.Context, driveID string, path string) (string, error)

    // Data I/O. Linux vfs_read + vfs_write.
    Read(ctx context.Context, driveID string, path string) ([]byte, error)
    Write(ctx context.Context, driveID string, path string, data []byte) error

    // Object data. Linux: open(O_RDONLY) returns fd; we return
    // a public ObjectRef for the caller to issue a presigned
    // download via the upload service.
    WriteObject(ctx context.Context, driveID string, path string, ref ObjectRef) error

    // Directory listing. Linux: iterate_dir.
    IterateDir(ctx context.Context, driveID string, path string) ([]DirEntry, error)

    // Drive CRUD (mdrive-specific).
    CreateDrive(...) (*Drive, error)
    GetDrive(...) (*Drive, error)
    GetDriveStorage(...) (*Storage, error)
    UpdateDrive(...) (*Drive, error)
    SoftDeleteDrive(...) error
    RestoreDrive(...) (*Drive, error)
    PurgeDrive(...) error
    ListDrives(...) ([]*Drive, error)
    ListDeletedDrives(...) ([]*Drive, error)
}
```

## vfs.NodeOperation (7 callbacks, Linux inode_operations)

```go
type NodeOperation interface {
    Lookup(ctx, dir *Node, name string) (*Dentry, error)
    Create(ctx, parent *Node, child *Node, name string) error
    Link(ctx, dentry *Dentry, linkDir *Node, linkName string) error
    Symlink(ctx, symlink *Node, target *Dentry) error
    Unlink(ctx, dentry *Dentry) error    // nlink-- + nlink==0 destroy
    Rmdir(ctx, dentry *Dentry) error
    Rename(ctx, old *Dentry, newDir *Node, newName string) error
}
```

## vfs.DriveOperation (9 methods, mdrive-specific)

```go
type DriveOperation interface {
    CreateDrive(...) (*Drive, error)
    GetDrive(...) (*Drive, error)
    GetDriveStorage(...) (*Storage, error)
    UpdateDrive(...) (*Drive, error)
    SoftDeleteDrive(...) error
    RestoreDrive(...) (*Drive, error)
    PurgeDrive(...) error
    ListDrivesByOwner(...) ([]*Drive, error)
    ListDeletedDrives(...) ([]*Drive, error)
}
```

## Path resolution (internal)

Inside vfs struct (unexported):

- `resolveTarget(ctx, driveID, path, action) (*Dentry, error)`
  — Walk to final node. Perm check per drive.
- `resolveParent(ctx, driveID, path, action) (*Dentry, string, error)`
  — Walk to parent + return basename.
- `step(ctx, cur, name, action) (*Dentry, error)` — one
  component; handles mount crossing + symlink follow.
- `followSymlink(ctx, cur, depth) (*Dentry, error)` —
  depth-capped at 8.
- `rootOf(ctx, driveID) (*Node, error)` — drive's root inode.
- `checkPerm(ctx, action, driveID) error` — entry-level perm.
- `splitPath`, `joinComponents` — path utilities.

## vfs.Dentry (Linux struct dentry simplified)

```go
type Dentry struct {
    Parent *Node
    Name   string
    Node   *Node
}
```

## vfs.content

- `internal/vfs/content/`: primitive structs only.
- `DirEntry.Kind` is `string`. Translate via `vfs.ParseNodeKind`.
- vfs reads/writes `content.*` via internal JSON marshalling.

## vfs.ObjectRef (public input)

```go
type ObjectRef struct {
    Bucket   string
    Key      string
    Mime     string
    Checksum string
}
```

Caller (handler, post-S3-upload) fills this in; vfs converts
to internal `content.ObjectContent` and stores inline.

## Read semantics (no open)

`Read(path) []byte` returns inline data of file-kind nodes.
Object-kind reads are not done here — handlers call
`upload.PresignDownload` for object download (a separate
syscall layer entry point). This mirrors the practical
division: Linux's `read(2)` is on a fd; our stateless HTTP
request path → bytes, and S3 objects go through presigned
URLs.

## Phase 0–3: completed

Phase 0 (cleanup) and Phase 1–3 (vfs interface, Resolver
inline, 22-method surface, all implementations) are complete.
22 commits ahead on `refactor/rebuild-core`.

## Out of scope for this PR

- `syscall/` package.
- `handler/` updates.
- `app/` wiring.
- mock regeneration.
- Garbage collection.
- Old `internal/vfs/drive/` removal.
- Test updates.
- `core/*` import cleanup.
- ent/drivestorage → ent/storage migration in remaining call sites.