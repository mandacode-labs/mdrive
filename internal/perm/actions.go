package perm

// Action is a typed enum for the OpenFGA relation strings.
// POSIX-aligned for read/write; storage-specific verbs for
// the rest.
type Action string

const (
	// ActionRead gates path resolution + read syscalls (Stat,
	// ReadFile, ReadObject, Getdents, PresignDownload).
	ActionRead Action = "can_read"

	// ActionWrite gates mutating syscalls on existing nodes
	// (WriteFile, Mkdir, SetTimes, CreateObject, LinkAt,
	// SymlinkAt, RenameAt).
	ActionWrite Action = "can_write"

	// ActionDelete gates unlink, rmdir, remove (cascading).
	ActionDelete Action = "can_delete"

	// ActionUpload gates presigned PUT URL issuance and the
	// upload-completion step that creates the object-kind node.
	ActionUpload Action = "can_upload"

	// ActionManageStorage gates drive.Storage configuration
	// changes (create/update/delete storage configs) and
	// bind-mount / umount.
	ActionManageStorage Action = "can_manage_storage"

	// ActionShareStorage gates access-grant operations on a
	// drive (future). Currently no fs.Service method uses it.
	ActionShareStorage Action = "can_share_storage"
)

// ObjectType is a typed enum for the OpenFGA object types.
type ObjectType string

const (
	ObjectTypeDrive ObjectType = "drive"
	ObjectTypeUser  ObjectType = "user"
)