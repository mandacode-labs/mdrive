package content

import (
	"encoding/json"

	"github.com/google/uuid"
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

type DirEntry struct {
	NodeID uuid.UUID `json:"ino"`
	Name   string    `json:"name"`
	Kind   NodeKind  `json:"kind"`
}

// DirContent is the JSON-serialized listing of a directory node.
// Stored inline in the node's content field.
type DirContent struct {
	Entries []DirEntry `json:"items"`
}

// Marshal implements [Content].
func (d *DirContent) Marshal() ([]byte, error) {
	return json.Marshal(d)
}

func (d *DirContent) AddEntry(entry DirEntry) error {
	// Check if the entry already exists
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

func NewDirContent(entries []DirEntry) Content {
	return &DirContent{Entries: entries}
}
