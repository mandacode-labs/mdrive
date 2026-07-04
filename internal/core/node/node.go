package node

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

// Content is the raw inline data of a node.
// Stored as a JSON-serialized blob in the node's content column, up to 4 KiB.
// Large data is stored externally (e.g., S3) and referenced via ObjectContent.
type Content []byte

// MaxContentSize is the maximum size of inline content.
const MaxContentSize = 4096

func (c Content) Size() int    { return len(c) }
func (c Content) Data() []byte { return c }

// Node is the in-memory inode abstraction. A node holds metadata
// and (for small items) inline content. The node does NOT know its
// drive, parent, or name — those live in the drive (root_node_id)
// and parent directory (DirContent), exactly as in Unix where
// i_parent and i_name are absent from the inode.
//
// Permission checks are not modeled here. OpenFGA owns access
// control across drives, and S3 (where applicable) owns per-object
// ACLs.
type Node struct {
	id      uuid.UUID
	kind    NodeKind
	size    int64
	nlink   uint32
	content Content
	atime   time.Time
	mtime   time.Time
	ctime   time.Time
	crtime  time.Time
	flags   Flags
	rev     Revision

	// staleRev is the revision loaded from the DB. Save uses it for
	// optimistic concurrency: UPDATE WHERE revision = staleRev.
	// On conflict (0 rows affected) it returns ErrRevisionConflict.
	staleRev Revision
}

// newNode creates a new Node. Private: external code must use type-specific constructors
// (NewFile, NewDirectory, NewSymlink, NewObject) which set the appropriate content.
//
// nlink starts at 0 (POSIX semantics): a freshly created inode has no hardlinks.
// The first successful Link sets nlink to 1; further Links increment; Unlink
// decrements and triggers deletion at nlink==0.
func newNode(kind NodeKind) *Node {
	now := time.Now()
	return &Node{
		id:      uuid.New(),
		kind:    kind,
		size:    0,
		nlink:   0,
		content: nil,
		atime:   now,
		mtime:   now,
		ctime:   now,
		crtime:  now,
		flags:   0,
		rev:     newRevision(),
	}
}

// NewRootNode creates a new root directory node for a drive.
// Public because root nodes have no parent. The drive package records
// this node's ID as drive.root_node_id after creation.
func NewRootNode() *Node {
	return newNode(NodeKindDirectory)
}

func (n *Node) ID() uuid.UUID  { return n.id }
func (n *Node) Kind() NodeKind { return n.kind }
func (n *Node) Size() int64    { return n.size }
func (n *Node) NLink() uint32  { return n.nlink }

// IncNLink increments the nlink counter by one. Mirrors the
// behavior of node.Service.Link's hardlink bookkeeping so test
// fakes (e.g. mock NodeClient callbacks) can keep the field
// consistent without reaching into unexported state.
func (n *Node) IncNLink()          { n.nlink++ }
func (n *Node) ATime() time.Time   { return n.atime }
func (n *Node) MTime() time.Time   { return n.mtime }
func (n *Node) CTime() time.Time   { return n.ctime }
func (n *Node) CRTime() time.Time  { return n.crtime }
func (n *Node) Flags() Flags       { return n.flags }
func (n *Node) Revision() Revision { return n.rev }

// StaleRev returns the revision that was current when the node was
// loaded from the repository. The repository uses it to detect concurrent
// modifications between Get and Save (optimistic concurrency).
func (n *Node) StaleRev() Revision { return n.staleRev }

// SetStaleRev records the revision that is currently persisted in the
// repository. It is called by Repository implementations after a
// successful Save to mark the in-memory node as in sync with storage.
// Callers should not invoke this directly; the repository owns the
// staleRev field.
func (n *Node) SetStaleRev(r Revision) { n.staleRev = r }

// Clone returns a deep copy of n. The content bytes are copied so
// mutating either side does not affect the other; the revision and
// staleRev are preserved (so this is a true snapshot of the node as
// the repository would hand it back, with no spurious revision bump).
// Repository implementations use this to materialize stored state
// without going through SetContent (which would increment rev).
func (n *Node) Clone() *Node {
	c := *n
	if n.content != nil {
		buf := make(Content, len(n.content))
		copy(buf, n.content)
		c.content = buf
	}
	return &c
}

// Content is exported for repository serialization; not intended for direct mutation
// by external callers. Type-specific Read methods are the public API.
func (n *Node) Content() Content { return n.content }

func (n *Node) IsDir() bool     { return n.kind == NodeKindDirectory }
func (n *Node) IsFile() bool    { return n.kind == NodeKindFile }
func (n *Node) IsSymlink() bool { return n.kind == NodeKindSymlink }
func (n *Node) IsObject() bool  { return n.kind == NodeKindObject }
func (n *Node) IsMount() bool   { return n.kind == NodeKindMount }

// write replaces the node's content and updates mtime/ctime/rev.
// Private: type-specific Write methods in file.go / dir.go / symlink.go / object.go
// marshal the appropriate content type and then call write.
func (n *Node) write(content Content, size int64) error {
	if len(content) > MaxContentSize {
		return errorx.New(errorx.KindInvalidArgument, "node: content exceeds maximum size")
	}
	n.content = content
	n.size = size
	now := time.Now()
	n.mtime = now
	n.ctime = now
	n.rev = n.rev.Next()
	return nil
}

// read returns the current content. Private: type-specific Read methods (ReadFile, etc.)
// unmarshal the content into a typed structure.
func (n *Node) read() (Content, error) {
	if n.content == nil {
		return nil, errorx.New(errorx.KindNotFound, "node: no content")
	}
	return n.content, nil
}
