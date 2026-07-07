package fs

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// NodeKind classifies an inode by storage shape. The kind
// decides how the node's data field is decoded on read and
// what content shape is accepted on write.
type NodeKind uint8

const (
	NodeKindFile      NodeKind = 0
	NodeKindDirectory NodeKind = 1
	NodeKindSymlink   NodeKind = 2
	NodeKindObject    NodeKind = 3
	NodeKindMount     NodeKind = 4
)

func (k NodeKind) String() string {
	switch k {
	case NodeKindFile:
		return "file"
	case NodeKindDirectory:
		return "directory"
	case NodeKindSymlink:
		return "symlink"
	case NodeKindObject:
		return "object"
	case NodeKindMount:
		return "mount"
	default:
		return "unknown"
	}
}

// Flags is a bitmask of node-level flags.
type Flags uint32

const (
	FlagSecureDeletion Flags = 0x00000001
	FlagUndelete       Flags = 0x00000002
	FlagCompress       Flags = 0x00000004
	FlagSynchronous    Flags = 0x00000008
	FlagImmutable      Flags = 0x00000010
	FlagAppendOnly     Flags = 0x00000020
	FlagNoDump         Flags = 0x00000040
	FlagNoAtime        Flags = 0x00000080
)

func (f Flags) UInt32() uint32 { return uint32(f) }

// Revision is a ULID-based generation counter for the node.
type Revision string

func newRevision() Revision {
	return Revision(ulid.Make().String())
}

func (r Revision) String() string { return string(r) }
func (r Revision) Next() Revision { return newRevision() }

// MaxDataSize bounds the inline data of a file-kind node.
const MaxDataSize = 4096

// Node is the inode-like record. All nodes of a drive share
// the same superblock id; the drive is reached via
// superblock → drive (one-to-one).
type Node struct {
	id    uuid.UUID
	sb    uuid.UUID
	kind  NodeKind
	size  int64
	nlink uint32
	data  []byte
	atime time.Time
	mtime time.Time
	ctime time.Time
	btime time.Time
	flags Flags
	rev   Revision
}

func (n *Node) ID() uuid.UUID           { return n.id }
func (n *Node) SuperblockID() uuid.UUID { return n.sb }
func (n *Node) Kind() NodeKind          { return n.kind }
func (n *Node) Size() int64             { return n.size }
func (n *Node) NLink() uint32           { return n.nlink }
func (n *Node) Data() []byte            { return n.data }
func (n *Node) ATime() time.Time        { return n.atime }
func (n *Node) MTime() time.Time        { return n.mtime }
func (n *Node) CTime() time.Time        { return n.ctime }
func (n *Node) BTime() time.Time        { return n.btime }
func (n *Node) Flags() Flags            { return n.flags }
func (n *Node) Revision() Revision      { return n.rev }

// NewNode constructs a fresh inode belonging to `sb`.
func NewNode(id uuid.UUID, sb uuid.UUID, kind NodeKind) *Node {
	rev := newRevision()
	return &Node{
		id:    id,
		sb:    sb,
		kind:  kind,
		flags: Flags(0),
		rev:   rev,
		atime: time.Now(),
		mtime: time.Now(),
		ctime: time.Now(),
		btime: time.Now(),
	}
}

// HydrateNode reconstructs a Node from its persisted fields.
func HydrateNode(
	id, sb uuid.UUID,
	kind NodeKind,
	size int64,
	nlink uint32,
	data []byte,
	atime, mtime, ctime, btime time.Time,
	flags Flags,
	rev Revision,
) *Node {
	return &Node{
		id: id, sb: sb, kind: kind, size: size, nlink: nlink,
		data: data, atime: atime, mtime: mtime, ctime: ctime, btime: btime,
		flags: flags, rev: rev,
	}
}

// Write sets the node's inline data and bumps revision.
// Enforces MaxDataSize.
func (n *Node) Write(data []byte, size int64) error {
	if len(data) > MaxDataSize {
		return errorx.New(errorx.KindInvalidArgument, "node: content exceeds maximum size")
	}
	n.data = data
	n.size = size
	now := time.Now()
	n.mtime = now
	n.ctime = now
	n.rev = n.rev.Next()
	return nil
}

// SetTimes is the user-visible timestamp setter (utimensat).
// Internal ops use NewNode/Write/IncNLink, which set their
// own timestamps. Service.SetTimes is the only legitimate
// caller.
func (n *Node) SetTimes(atime, mtime, ctime, btime time.Time) {
	n.atime = atime
	n.mtime = mtime
	n.ctime = ctime
	n.btime = btime
	n.rev = n.rev.Next()
}

func (n *Node) IncNLink() {
	now := time.Now()
	n.nlink++
	n.ctime = now
	n.rev = n.rev.Next()
}

func (n *Node) DecNLink() {
	if n.nlink > 0 {
		now := time.Now()
		n.nlink--
		n.ctime = now
		n.rev = n.rev.Next()
	}
}

// NodeOperation is the inode-level callback set. Mirrors
// Linux inode_operations. Methods that need parent context
// take *Dentry (matching Linux's parent dentry); operations
// that only need the inode take *Node.
type NodeOperation interface {
	Get(ctx context.Context, id uuid.UUID) (*Node, error)
	Lookup(ctx context.Context, parent *Dentry, name string) (*Dentry, error)
	Create(ctx context.Context, parent *Node, child *Node, name string) error
	Persist(ctx context.Context, n *Node) error
	Link(ctx context.Context, dentry *Dentry, linkDir *Dentry, linkName string) error
	Unlink(ctx context.Context, dentry *Dentry) error
	Rmdir(ctx context.Context, dentry *Dentry) error
	Rename(ctx context.Context, old *Dentry, newParent *Dentry, newName string) error
}