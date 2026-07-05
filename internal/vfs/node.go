package vfs

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// NodeKind classifies a node by storage shape, not by access
// permissions (those are managed via OpenFGA).
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

// Flags is a bitmask of node-level flags (ext2-style i_flags).
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

func (f *Flags) UInt32() uint32 {
	return uint32(*f)
}

func (f *Flags) Set(flag Flags) {
	*f |= flag
}

func (f *Flags) Clear(flag Flags) {
	*f &^= flag
}

func (f *Flags) Has(flag Flags) bool {
	return (*f & flag) != 0
}

func (f Flags) String() string {
	var flags []string
	if f.Has(FlagSecureDeletion) {
		flags = append(flags, "secure_deletion")
	}
	if f.Has(FlagUndelete) {
		flags = append(flags, "undelete")
	}
	if f.Has(FlagCompress) {
		flags = append(flags, "compress")
	}
	if f.Has(FlagSynchronous) {
		flags = append(flags, "synchronous")
	}
	if f.Has(FlagImmutable) {
		flags = append(flags, "immutable")
	}
	if f.Has(FlagAppendOnly) {
		flags = append(flags, "append_only")
	}
	if f.Has(FlagNoDump) {
		flags = append(flags, "no_dump")
	}
	if f.Has(FlagNoAtime) {
		flags = append(flags, "no_atime")
	}
	if len(flags) == 0 {
		return "none"
	}
	return fmt.Sprintf("%v", flags)
}

// Revision is a ULID-based generation identifier for the node.
// Used for optimistic concurrency control (analogous to inode i_generation in Linux).
type Revision string

func newRevision() Revision {
	return Revision(ulid.Make().String())
}

func (r Revision) String() string {
	return string(r)
}

func (r Revision) Equals(other Revision) bool {
	return r == other
}

func (r Revision) IsEmpty() bool {
	return r == ""
}

func (r Revision) Next() Revision {
	return newRevision()
}

func (r Revision) Time() (time.Time, error) {
	id, err := ulid.Parse(string(r))
	if err != nil {
		return time.Time{}, err
	}
	return ulid.Time(id.Time()), nil
}

func (n *Node) IsDir() bool     { return n.kind == NodeKindDirectory }
func (n *Node) IsFile() bool    { return n.kind == NodeKindFile }
func (n *Node) IsSymlink() bool { return n.kind == NodeKindSymlink }
func (n *Node) IsObject() bool  { return n.kind == NodeKindObject }
func (n *Node) IsMount() bool   { return n.kind == NodeKindMount }

// MaxDataSize is the maximum size of data
const MaxDataSize = 4096

// Node represents a file system node
type Node struct {
	id     uuid.UUID
	drv    string // Drive ID
	kind   NodeKind
	size   int64
	nlink  uint32
	data   []byte
	atime  time.Time
	mtime  time.Time
	ctime  time.Time
	crtime time.Time
	flags  Flags
	rev    Revision

	// staleRev is the revision loaded from the DB. Save uses it for
	// optimistic concurrency: UPDATE WHERE revision = staleRev.
	// On conflict (0 rows affected) it returns ErrRevisionConflict.
	staleRev Revision
}

// Getters
func (n *Node) ID() uuid.UUID      { return n.id }
func (n *Node) DriveID() string    { return n.drv }
func (n *Node) Kind() NodeKind     { return n.kind }
func (n *Node) Size() int64        { return n.size }
func (n *Node) NLink() uint32      { return n.nlink }
func (n *Node) Data() []byte       { return n.data }
func (n *Node) ATime() time.Time   { return n.atime }
func (n *Node) MTime() time.Time   { return n.mtime }
func (n *Node) CTime() time.Time   { return n.ctime }
func (n *Node) CRTime() time.Time  { return n.crtime }
func (n *Node) Flags() Flags       { return n.flags }
func (n *Node) Revision() Revision { return n.rev }
func (n *Node) StaleRev() Revision { return n.staleRev }

func (n *Node) IncNLink() { n.nlink++ }
func (n *Node) DecNLink() {
	if n.nlink > 0 {
		n.nlink--
	}
}

func (n *Node) SetData(data []byte)    { n.data = data }
func (n *Node) SetNLink(nlink uint32)  { n.nlink = nlink }
func (n *Node) SetSize(size int64)     { n.size = size }
func (n *Node) SetMTime(t time.Time)   { n.mtime = t }
func (n *Node) SetCTime(t time.Time)   { n.ctime = t }
func (n *Node) SetATime(t time.Time)   { n.atime = t }
func (n *Node) SetCRTime(t time.Time)  { n.crtime = t }
func (n *Node) SetFlags(f Flags)       { n.flags = f }
func (n *Node) SetRevision(r Revision) { n.rev = r }
func (n *Node) SetStaleRev(r Revision) { n.staleRev = r }

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

func NewNode(id uuid.UUID, drv string, kind NodeKind) *Node {
	rev := newRevision()
	return &Node{
		id:     id,
		drv:    drv,
		kind:   kind,
		size:   0,
		nlink:  0,
		data:   nil,
		atime:  time.Now(),
		mtime:  time.Now(),
		ctime:  time.Now(),
		crtime: time.Now(),
		flags:  Flags(0),
		rev:    rev,
	}
}
