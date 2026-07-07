package fs

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// DirEntry is a single directory listing row. Same shape on
// disk (via DirContent) and over the wire (Service.Getdents
// returns *DirContent whose Entries are []DirEntry).
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