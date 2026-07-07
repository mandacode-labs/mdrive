package fs

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// Content is the JSON-marshaled inline payload of a node.
// Each NodeKind has its own concrete payload type; the kind
// field on the parent Node picks the decoder.
type Content interface {
	Marshal() ([]byte, error)
}

// FileContent is the inline payload of a file-kind node,
// JSON-marshaled in the node's data field up to MaxDataSize
// (4KB). Mime/Encoding/Checksum ride with the body so callers
// don't need a second roundtrip to stat the file.
type FileContent struct {
	Raw      string `json:"raw"`
	Mime     string `json:"mime,omitempty"`
	Encoding string `json:"enc,omitempty"`
	Checksum string `json:"sum,omitempty"`
}

func (f *FileContent) Marshal() ([]byte, error) { return json.Marshal(f) }

// ObjectContent is the inline payload of an object-kind node.
// The actual blob lives in S3; this struct carries the S3
// reference plus the metadata callers need.
type ObjectContent struct {
	Bucket   string `json:"bucket"`
	Key      string `json:"key"`
	Mime     string `json:"mime"`
	Checksum string `json:"sum,omitempty"`
	Size     int64  `json:"size"`
}

func (o *ObjectContent) Marshal() ([]byte, error) { return json.Marshal(o) }

// SymlinkContent is the inline payload of a symlink-kind node:
// the target's inode id. The VFS layer resolves it.
type SymlinkContent struct {
	NodeID uuid.UUID `json:"target"`
}

func (s *SymlinkContent) Marshal() ([]byte, error) { return json.Marshal(s) }

// MountContent is the inline payload of a mount-kind node:
// the source drive id. The VFS layer looks up the source's
// superblock and follows.
type MountContent struct {
	DriveID string `json:"src"`
}

func (m *MountContent) Marshal() ([]byte, error) { return json.Marshal(m) }

// DirEntry is a single directory listing row.
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

func (d *DirContent) Marshal() ([]byte, error) { return json.Marshal(d) }

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
