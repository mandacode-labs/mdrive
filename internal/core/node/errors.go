package node

import "errors"

// Node-domain sentinel errors. Use errors.Is(err, node.ErrXxx) to check.
var (
	// ErrNotFound is returned when a node is not present in the repository.
	ErrNotFound = errors.New("node: not found")

	// ErrEntryExists is returned when adding a directory entry that already exists.
	ErrEntryExists = errors.New("node: entry already exists")

	// ErrEntryNotFound is returned when removing or looking up a directory entry that does not exist.
	ErrEntryNotFound = errors.New("node: entry not found")

	// ErrNotDirectory is returned when an operation requires a directory node.
	ErrNotDirectory = errors.New("node: not a directory")

	// ErrInvalidType is returned when an operation is attempted on the wrong node type.
	ErrInvalidType = errors.New("node: invalid type for operation")

	// ErrInvalidName is returned when a name is empty or otherwise invalid.
	ErrInvalidName = errors.New("node: invalid name")

	// ErrInvalidReference is returned when an object reference is missing required fields.
	ErrInvalidReference = errors.New("node: invalid object reference")

	// ErrInvalidSize is returned when a size value is negative.
	ErrInvalidSize = errors.New("node: invalid size")

	// ErrNoContent is returned when reading a node that has no inline content.
	ErrNoContent = errors.New("node: no content")

	// ErrContentTooLarge is returned when content exceeds MaxContentSize.
	ErrContentTooLarge = errors.New("node: content exceeds maximum size")

	// ErrRevisionConflict is returned when a Save detects that the node's
	// revision has changed since it was loaded (concurrent update).
	ErrRevisionConflict = errors.New("node: revision conflict")

	// ErrIsDirectory is returned when overwriting/replacing an entry that
	// points to a directory (POSIX: cannot overwrite a directory with a
	// non-directory).
	ErrIsDirectory = errors.New("node: target is a directory")

	// ErrIsObject is returned when a caller asks vfs to inline the
	// bytes of an S3-backed object node. vfs is the inode-tree
	// manager and does not do S3 I/O; object-node bytes are reached
	// through the upload presign-download flow.
	ErrIsObject = errors.New("node: target is an S3 object; use the presign-download endpoint")

	// ErrInvalidMoveOverwrite is returned when a move would overwrite
	// an entry whose type does not match the source (e.g. moving a
	// file onto a directory or vice versa).
	ErrInvalidMoveOverwrite = errors.New("node: cannot overwrite entry of different type")

	// ErrSymlinkCycle is returned when symlink resolution exceeds the
	// hop limit (POSIX ELOOP). Mirrors Linux's MAXSYMLINKS budget.
	ErrSymlinkCycle = errors.New("node: symlink cycle or too many hops")
)
