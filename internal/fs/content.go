package fs

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

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

// Content is the JSON-marshaled inline payload of a node.
// Each NodeKind has its own concrete payload type; the kind
// field on the parent Node picks the decoder.
type Content interface {
	Marshal() ([]byte, error)
}

// FileContent is the inline payload of a small file-kind
// node, stored JSON-marshaled in the node's data field up
// to MaxDataSize (4KB). Mime / Encoding / Checksum ride
// along with the body so callers don't need a second
// roundtrip to stat the file.
type FileContent struct {
	Raw      string `json:"raw"`
	Mime     string `json:"mime,omitempty"`
	Encoding string `json:"enc,omitempty"`
	Checksum string `json:"sum,omitempty"`
}

func (f *FileContent) Marshal() ([]byte, error)     { return json.Marshal(f) }
func NewFileContent(raw string) Content            { return &FileContent{Raw: raw} }

// ObjectContent is the inline payload of an object-kind
// node. The actual blob lives in S3; this struct carries
// the S3 reference plus the metadata callers need without
// a second roundtrip to the bucket.
type ObjectContent struct {
	Bucket   string `json:"bucket"`
	Key      string `json:"key"`
	Mime     string `json:"mime"`
	Checksum string `json:"sum,omitempty"`
	Size     int64  `json:"size"`
}

func (o *ObjectContent) Marshal() ([]byte, error) {
	return json.Marshal(o)
}
func NewObjectContent(bucket, key, mime, checksum string, size int64) Content {
	return &ObjectContent{Bucket: bucket, Key: key, Mime: mime, Checksum: checksum, Size: size}
}

// SymlinkContent is the inline payload of a symlink-kind
// node: just the target's inode id. The VFS layer is
// responsible for resolving it.
type SymlinkContent struct {
	NodeID uuid.UUID `json:"target"`
}

func (s *SymlinkContent) Marshal() ([]byte, error) { return json.Marshal(s) }
func NewSymlinkContent(nodeID uuid.UUID) Content   { return &SymlinkContent{NodeID: nodeID} }

// MountContent is the inline payload of a mount-kind node:
// the source drive id this node points at. The VFS layer
// looks up the source's superblock and follows.
type MountContent struct {
	DriveID string `json:"src"`
}

func (m *MountContent) Marshal() ([]byte, error) { return json.Marshal(m) }
func NewMountContent(driveID string) Content     { return &MountContent{DriveID: driveID} }

// DirEntry is a single directory listing row. Same shape
// on disk (via DirContent) and over the wire
// (Service.Getdents returns *DirContent whose Entries are
// []DirEntry).
type DirEntry struct {
	NodeID uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Kind   NodeKind  `json:"kind"`
}

// DirContent is the JSON-serialized listing of a directory
// node. Stored inline in the node's data field.
type DirContent struct {
	Entries []DirEntry `json:"entries"`
}

func (d *DirContent) Marshal() ([]byte, error) {
	return json.Marshal(d)
}

func (d *DirContent) AddEntry(entry DirEntry) error {
	for i := range d.Entries {
		if d.Entries[i].Name == entry.Name {
			return errorx.New(errorx.KindAlreadyExists, "entry already exists")
		}
	}
	d.Entries = append(d.Entries, entry)
	return nil
}

func (d *DirContent) FindEntry(name string) *DirEntry {
	for i := range d.Entries {
		if d.Entries[i].Name == name {
			return &d.Entries[i]
		}
	}
	return nil
}

func (d *DirContent) RemoveEntry(name string) error {
	for i := range d.Entries {
		if d.Entries[i].Name == name {
			d.Entries = append(d.Entries[:i], d.Entries[i+1:]...)
			return nil
		}
	}
	return errorx.New(errorx.KindNotFound, "entry not found")
}

// Ensure time package is referenced when types are unused
// in a build; keeps imports stable across future edits.
var _ = time.Time{}
var _ = fmt.Sprintf