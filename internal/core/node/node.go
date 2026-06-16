package node

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

// NodeType represents the type of a node.
type NodeType uint8

const (
	NodeTypeFile      NodeType = 0
	NodeTypeDirectory NodeType = 1
	NodeTypeSymlink   NodeType = 2
	NodeTypeObject    NodeType = 3
	NodeTypeDevice    NodeType = 4
)

func (nt NodeType) String() string {
	switch nt {
	case NodeTypeFile:
		return "file"
	case NodeTypeDirectory:
		return "directory"
	case NodeTypeSymlink:
		return "symlink"
	case NodeTypeObject:
		return "object"
	case NodeTypeDevice:
		return "device"
	default:
		return "unknown"
	}
}

func (nt NodeType) Equals(other NodeType) bool {
	return nt == other
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

func (c Content) Size() int     { return len(c) }
func (c Content) Data() []byte   { return c }

// Node is the POSIX-style inode abstraction.
// A node holds metadata and (for small items) inline content. The node does NOT know its
// drive, parent, or name — those live in the drive (root_node_id) and parent directory
// (DirContent), exactly as in Unix where i_parent and i_name are absent from the inode.
//
// S3 data state is NOT tracked here. For object nodes, the actual S3 data
// existence is checked lazily on read (HEAD to S3). If the S3 object is
// missing, the read returns ErrNoContent (or the caller maps it to 404).
type Node struct {
	id      uuid.UUID
	typ     NodeType
	size    int64
	nlink   uint32
	content Content
	atime   time.Time
	mtime   time.Time
	ctime   time.Time
	crtime  time.Time
	flags   Flags
	rev     Revision
}

// newNode creates a new Node. Private: external code must use type-specific constructors
// (NewFile, NewDirectory, NewSymlink, NewObject) which set the appropriate content.
func newNode(typ NodeType) *Node {
	now := time.Now()
	return &Node{
		id:      uuid.New(),
		typ:     typ,
		size:    0,
		nlink:   1,
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
	return newNode(NodeTypeDirectory)
}

// Getters (all public).

func (n *Node) ID() uuid.UUID      { return n.id }
func (n *Node) Type() NodeType     { return n.typ }
func (n *Node) Size() int64        { return n.size }
func (n *Node) NLink() uint32      { return n.nlink }
func (n *Node) ATime() time.Time   { return n.atime }
func (n *Node) MTime() time.Time   { return n.mtime }
func (n *Node) CTime() time.Time   { return n.ctime }
func (n *Node) CRTime() time.Time  { return n.crtime }
func (n *Node) Flags() Flags       { return n.flags }
func (n *Node) Revision() Revision { return n.rev }

// Content is exported for repository serialization; not intended for direct mutation
// by external callers. Type-specific Read methods are the public API.
func (n *Node) Content() Content { return n.content }

// Type predicates.

func (n *Node) IsDir() bool     { return n.typ == NodeTypeDirectory }
func (n *Node) IsFile() bool    { return n.typ == NodeTypeFile }
func (n *Node) IsSymlink() bool { return n.typ == NodeTypeSymlink }
func (n *Node) IsObject() bool  { return n.typ == NodeTypeObject }

// write replaces the node's content and updates mtime/ctime/rev.
// Private: type-specific Write methods in file.go / dir.go / symlink.go / object.go
// marshal the appropriate content type and then call write.
func (n *Node) write(content Content, size int64) error {
	if len(content) > MaxContentSize {
		return ErrContentTooLarge
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
		return nil, ErrNoContent
	}
	return n.content, nil
}

// SetSize updates the size field and bumps mtime/ctime/rev.
func (n *Node) SetSize(size int64) {
	n.size = size
	now := time.Now()
	n.mtime = now
	n.ctime = now
	n.rev = n.rev.Next()
}
