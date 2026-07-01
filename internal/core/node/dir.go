package node

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mandacode-labs/mdrive/internal/errorx"
)

// DirEntry follows the Linux struct linux_dirent pattern: an inode reference plus a name.
// We additionally include the type for ls-style listing (Linux exposes d_type for this).
// JSON tags are kept short to minimize inline content size.
type DirEntry struct {
	InodeID uuid.UUID `json:"ino"`
	Name    string    `json:"name"`
	Type    NodeType  `json:"type"`
}

// DirContent is the JSON-serialized listing of a directory node.
// Stored inline in the node's content field.
type DirContent struct {
	Entries []DirEntry `json:"items"`
}

// NewDirContent creates a DirContent from a list of entries.
func NewDirContent(entries []DirEntry) *DirContent {
	return &DirContent{Entries: entries}
}

// Marshal returns the JSON representation of DirContent.
func (d *DirContent) Marshal() ([]byte, error) {
	return json.Marshal(d)
}

// findEntry returns the entry with the given name, or nil if not present.
func (d *DirContent) findEntry(name string) *DirEntry {
	for i := range d.Entries {
		if d.Entries[i].Name == name {
			return &d.Entries[i]
		}
	}
	return nil
}

// NewDirectory creates a new empty directory node.
func NewDirectory() (*Node, error) {
	data, err := json.Marshal(NewDirContent(nil))
	if err != nil {
		return nil, errorx.Wrap(err, "node: marshal directory content")
	}
	n := newNode(NodeTypeDirectory)
	if err := n.write(Content(data), 0); err != nil {
		return nil, err
	}
	return n, nil
}

// ReadDir returns the directory listing.
func (n *Node) ReadDir() (DirContent, error) {
	content, err := n.read()
	if err != nil {
		return DirContent{}, errorx.Wrap(err, "node: read directory content")
	}
	var dc DirContent
	if err := json.Unmarshal(content, &dc); err != nil {
		return DirContent{}, errorx.Wrap(err, "node: unmarshal directory content")
	}
	return dc, nil
}

// WriteDir replaces the directory's content with the given listing.
func (n *Node) WriteDir(dc DirContent) error {
	if n.kind != NodeTypeDirectory {
		return ErrNotDirectory
	}
	data, err := json.Marshal(&dc)
	if err != nil {
		return errorx.Wrap(err, "node: marshal directory content (entries=%d)", len(dc.Entries))
	}
	if len(data) > MaxContentSize {
		return ErrContentTooLarge
	}
	return n.write(Content(data), int64(len(data)))
}

// AddEntry adds a child entry to the directory.
// Fails if the entry already exists or the node is not a directory.
func (n *Node) AddEntry(name string, child *Node) error {
	if n.kind != NodeTypeDirectory {
		return ErrNotDirectory
	}
	if name == "" {
		return ErrInvalidName
	}
	dc, err := n.ReadDir()
	if err != nil {
		return err
	}
	if dc.findEntry(name) != nil {
		return ErrEntryExists
	}
	dc.Entries = append(dc.Entries, DirEntry{
		InodeID: child.id,
		Name:    name,
		Type:    child.kind,
	})
	return n.WriteDir(dc)
}

// AddEntries adds multiple child entries in a single DirContent write,
// persisting the directory exactly once. Fails atomically: if any name
// is empty or already present, no entries are added.
func (n *Node) AddEntries(entries map[string]*Node) error {
	if n.kind != NodeTypeDirectory {
		return ErrNotDirectory
	}
	if len(entries) == 0 {
		return nil
	}
	dc, err := n.ReadDir()
	if err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(dc.Entries))
	for _, e := range dc.Entries {
		existing[e.Name] = struct{}{}
	}
	for name, child := range entries {
		if name == "" {
			return ErrInvalidName
		}
		if child == nil {
			return errorx.New(errorx.KindBadRequest, "node: add entries: nil child for "+name)
		}
		if _, ok := existing[name]; ok {
			return ErrEntryExists
		}
		dc.Entries = append(dc.Entries, DirEntry{
			InodeID: child.id,
			Name:    name,
			Type:    child.kind,
		})
	}
	return n.WriteDir(dc)
}

// RemoveEntry removes a child entry by name.
func (n *Node) RemoveEntry(name string) error {
	if n.kind != NodeTypeDirectory {
		return ErrNotDirectory
	}
	dc, err := n.ReadDir()
	if err != nil {
		return err
	}
	for i := range dc.Entries {
		if dc.Entries[i].Name == name {
			dc.Entries = append(dc.Entries[:i], dc.Entries[i+1:]...)
			return n.WriteDir(dc)
		}
	}
	return ErrEntryNotFound
}

// RemoveEntries removes multiple child entries in a single DirContent
// write. Entries that do not exist are silently skipped (POSIX rm -f
// semantics) so partial failure is acceptable; the directory is
// persisted exactly once.
func (n *Node) RemoveEntries(names []string) error {
	if n.kind != NodeTypeDirectory {
		return ErrNotDirectory
	}
	if len(names) == 0 {
		return nil
	}
	dc, err := n.ReadDir()
	if err != nil {
		return err
	}
	toRemove := make(map[string]struct{}, len(names))
	for _, n := range names {
		toRemove[n] = struct{}{}
	}
	out := dc.Entries[:0]
	for _, e := range dc.Entries {
		if _, drop := toRemove[e.Name]; !drop {
			out = append(out, e)
		}
	}
	if len(out) == len(dc.Entries) {
		return nil // nothing removed
	}
	dc.Entries = out
	return n.WriteDir(dc)
}

// Lookup returns the child entry with the given name, or nil if not present.
func (n *Node) Lookup(name string) (*DirEntry, error) {
	if n.kind != NodeTypeDirectory {
		return nil, ErrNotDirectory
	}
	dc, err := n.ReadDir()
	if err != nil {
		return nil, err
	}
	return dc.findEntry(name), nil
}
