package node

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestNewFile(t *testing.T) {
	n, err := NewFile("hello world")
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	if n.Type() != NodeTypeFile {
		t.Errorf("expected type file, got %v", n.Type())
	}
	if n.Size() != int64(len("hello world")) {
		t.Errorf("expected size %d, got %d", len("hello world"), n.Size())
	}
	if n.NLink() != 1 {
		t.Errorf("expected nlink 1, got %d", n.NLink())
	}
	if n.Revision().IsEmpty() {
		t.Error("expected non-empty revision")
	}
}

func TestNewFile_Empty(t *testing.T) {
	n, err := NewFile("")
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	if n.Size() != 0 {
		t.Errorf("expected size 0, got %d", n.Size())
	}
}

func TestNewFile_TooLarge(t *testing.T) {
	large := make([]byte, MaxContentSize+1)
	for i := range large {
		large[i] = 'a'
	}
	_, err := NewFile(string(large))
	if err == nil {
		t.Error("expected error for content too large")
	}
}

func TestFileReadWrite(t *testing.T) {
	n, err := NewFile("initial content")
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	initialRev := n.Revision()

	if err := n.WriteFile("updated content"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := n.ReadFile()
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got != "updated content" {
		t.Errorf("expected 'updated content', got %q", got)
	}
	if n.Revision() == initialRev {
		t.Error("expected revision to change after WriteFile")
	}
}

func TestNewDirectory(t *testing.T) {
	n, err := NewDirectory()
	if err != nil {
		t.Fatalf("NewDirectory: %v", err)
	}
	if n.Type() != NodeTypeDirectory {
		t.Errorf("expected type directory, got %v", n.Type())
	}
	entries, err := n.ReadDir()
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries.Entries) != 0 {
		t.Errorf("expected empty directory, got %d entries", len(entries.Entries))
	}
}

func TestDirectoryAddRemoveEntry(t *testing.T) {
	dir, err := NewDirectory()
	if err != nil {
		t.Fatalf("NewDirectory: %v", err)
	}
	child, err := NewFile("file content")
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}

	if err := dir.AddEntry("foo.txt", child); err != nil {
		t.Fatalf("AddEntry: %v", err)
	}
	entries, err := dir.ReadDir()
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries.Entries))
	}
	if entries.Entries[0].Name != "foo.txt" {
		t.Errorf("expected name 'foo.txt', got %q", entries.Entries[0].Name)
	}
	if entries.Entries[0].InodeID != child.ID() {
		t.Errorf("expected InodeID %v, got %v", child.ID(), entries.Entries[0].InodeID)
	}
	if entries.Entries[0].Type != NodeTypeFile {
		t.Errorf("expected type file, got %v", entries.Entries[0].Type)
	}

	// Adding the same name again should fail
	if err := dir.AddEntry("foo.txt", child); err != ErrEntryExists {
		t.Errorf("expected ErrEntryExists, got %v", err)
	}

	// Lookup
	e, err := dir.Lookup("foo.txt")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if e == nil {
		t.Fatal("expected entry, got nil")
	}

	// Remove
	if err := dir.RemoveEntry("foo.txt"); err != nil {
		t.Fatalf("RemoveEntry: %v", err)
	}
	entries, err = dir.ReadDir()
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries.Entries) != 0 {
		t.Errorf("expected 0 entries after remove, got %d", len(entries.Entries))
	}

	// Remove non-existent
	if err := dir.RemoveEntry("nope"); err != ErrEntryNotFound {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}
}

func TestAddEntry_NotDirectory(t *testing.T) {
	file, err := NewFile("x")
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	child, err := NewFile("y")
	if err != nil {
		t.Fatalf("NewFile: %v", err)
	}
	if err := file.AddEntry("foo", child); err != ErrNotDirectory {
		t.Errorf("expected ErrNotDirectory, got %v", err)
	}
}

func TestAddEntry_EmptyName(t *testing.T) {
	dir, _ := NewDirectory()
	child, _ := NewFile("x")
	if err := dir.AddEntry("", child); err != ErrInvalidName {
		t.Errorf("expected ErrInvalidName, got %v", err)
	}
}

func TestNewSymlink(t *testing.T) {
	n, err := NewSymlink("/target/path")
	if err != nil {
		t.Fatalf("NewSymlink: %v", err)
	}
	if n.Type() != NodeTypeSymlink {
		t.Errorf("expected type symlink, got %v", n.Type())
	}
	target, err := n.ReadSymlink()
	if err != nil {
		t.Fatalf("ReadSymlink: %v", err)
	}
	if target != "/target/path" {
		t.Errorf("expected '/target/path', got %q", target)
	}
}

func TestNewSymlink_Empty(t *testing.T) {
	if _, err := NewSymlink(""); err != ErrInvalidName {
		t.Errorf("expected ErrInvalidName, got %v", err)
	}
}

func TestSymlinkUpdate(t *testing.T) {
	n, _ := NewSymlink("/old")
	if err := n.WriteSymlink("/new"); err != nil {
		t.Fatalf("WriteSymlink: %v", err)
	}
	target, _ := n.ReadSymlink()
	if target != "/new" {
		t.Errorf("expected '/new', got %q", target)
	}
}

func TestNewObject(t *testing.T) {
	oc := NewObjectContent("my-bucket", "path/to/key", "text/plain", "abc123")
	n, err := NewObject(*oc, 1024)
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	if n.Type() != NodeTypeObject {
		t.Errorf("expected type object, got %v", n.Type())
	}
	if n.Size() != 1024 {
		t.Errorf("expected size 1024, got %d", n.Size())
	}
	got, err := n.ReadObject()
	if err != nil {
		t.Fatalf("ReadObject: %v", err)
	}
	if got.Bucket != "my-bucket" || got.Key != "path/to/key" {
		t.Errorf("unexpected object content: %+v", got)
	}
	if got.Mime != "text/plain" {
		t.Errorf("expected Mime 'text/plain', got %q", got.Mime)
	}
	if got.Checksum != "abc123" {
		t.Errorf("expected Checksum 'abc123', got %q", got.Checksum)
	}
}

func TestDirContent_JSONTags(t *testing.T) {
	// Verify short JSON tags are applied (saves content bytes).
	dc := DirContent{
		Entries: []DirEntry{
			{InodeID: uuidFromString(t, "550e8400-e29b-41d4-a716-446655440000"), Name: "x", Type: NodeTypeFile},
		},
	}
	data, err := jsonMarshal(dc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Should contain "ino" not "inode_id"
	if !contains(data, []byte(`"ino":`)) {
		t.Errorf("expected short tag 'ino' in JSON, got: %s", data)
	}
	if contains(data, []byte(`"inode_id"`)) {
		t.Errorf("should not contain long tag 'inode_id' in JSON, got: %s", data)
	}
	// Should contain "items" not "entries"
	if !contains(data, []byte(`"items":`)) {
		t.Errorf("expected short tag 'items' in JSON, got: %s", data)
	}
}

func TestObjectContent_JSONTags(t *testing.T) {
	oc := ObjectContent{
		Bucket:   "b",
		Key:      "k",
		Mime:     "text/plain",
		Checksum: "abc",
	}
	data, err := jsonMarshal(oc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !contains(data, []byte(`"mime":`)) {
		t.Errorf("expected 'mime' tag, got: %s", data)
	}
	if !contains(data, []byte(`"sum":`)) {
		t.Errorf("expected 'sum' tag, got: %s", data)
	}
	if contains(data, []byte(`"content_type"`)) {
		t.Errorf("should not contain 'content_type' tag, got: %s", data)
	}
	if contains(data, []byte(`"checksum"`)) {
		t.Errorf("should not contain 'checksum' tag, got: %s", data)
	}
}

func TestFileContent_JSONTag(t *testing.T) {
	fc := FileContent{Raw: "hello"}
	data, err := jsonMarshal(fc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// "raw" should be present
	if !contains(data, []byte(`"raw":`)) {
		t.Errorf("expected 'raw' tag, got: %s", data)
	}
}

func TestSymlinkContent_JSONTag(t *testing.T) {
	sc := SymlinkContent{Target: "/path"}
	data, err := jsonMarshal(sc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !contains(data, []byte(`"target":`)) {
		t.Errorf("expected 'target' tag, got: %s", data)
	}
}

// jsonMarshal is a small helper that uses encoding/json.
func jsonMarshal(v any) ([]byte, error) {
	return jsonMarshalImpl(v)
}

// jsonMarshalImpl is the actual implementation (so callers can be stubbed if needed).
func jsonMarshalImpl(v any) ([]byte, error) {
	return json.Marshal(v)
}

func contains(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}

func uuidFromString(t *testing.T, s string) uuid.UUID {
	t.Helper()
	u, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("invalid uuid: %v", err)
	}
	return u
}

func TestNewObject_InvalidRef(t *testing.T) {
	_, err := NewObject(ObjectContent{Bucket: "", Key: "k"}, 100)
	if err != ErrInvalidReference {
		t.Errorf("expected ErrInvalidReference, got %v", err)
	}
	_, err = NewObject(ObjectContent{Bucket: "b", Key: ""}, 100)
	if err != ErrInvalidReference {
		t.Errorf("expected ErrInvalidReference, got %v", err)
	}
}

func TestNewObject_NegativeSize(t *testing.T) {
	oc := NewObjectContent("b", "k", "text/plain", "")
	_, err := NewObject(*oc, -1)
	if err != ErrInvalidSize {
		t.Errorf("expected ErrInvalidSize, got %v", err)
	}
}

func TestContentSize(t *testing.T) {
	c := Content([]byte("hello"))
	if c.Size() != 5 {
		t.Errorf("expected size 5, got %d", c.Size())
	}
}

func TestWriteContentTooLarge(t *testing.T) {
	n, _ := NewFile("x")
	large := make([]byte, MaxContentSize+1)
	if err := n.write(large, int64(len(large))); err != ErrContentTooLarge {
		t.Errorf("expected ErrContentTooLarge, got %v", err)
	}
}

func TestNewRootNode(t *testing.T) {
	n := NewRootNode()
	if n.Type() != NodeTypeDirectory {
		t.Errorf("expected type directory, got %v", n.Type())
	}
}

func TestRevision(t *testing.T) {
	r1 := newRevision()
	r2 := r1.Next()
	if r1 == r2 {
		t.Error("expected revisions to differ")
	}
	if r1.IsEmpty() {
		t.Error("expected non-empty revision")
	}
	empty := Revision("")
	if !empty.IsEmpty() {
		t.Error("expected empty revision")
	}
}

func TestFlags(t *testing.T) {
	var f Flags
	if f.Has(FlagImmutable) {
		t.Error("expected flag not set initially")
	}
	f.Set(FlagImmutable)
	if !f.Has(FlagImmutable) {
		t.Error("expected flag to be set after Set")
	}
	f.Set(FlagNoAtime)
	if !f.Has(FlagImmutable) || !f.Has(FlagNoAtime) {
		t.Error("expected both flags to be set")
	}
	f.Clear(FlagImmutable)
	if f.Has(FlagImmutable) {
		t.Error("expected flag to be cleared")
	}
	if !f.Has(FlagNoAtime) {
		t.Error("expected other flag to remain set")
	}
}
