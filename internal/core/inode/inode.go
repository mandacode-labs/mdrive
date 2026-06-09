package inode

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/mandacode-labs/retrowin-go/internal/core/inode/content"
	"github.com/mandacode-labs/retrowin-go/internal/errors"
)

// Mode constants following Linux inode conventions.
// The mode field encodes both file type (upper bits) and permissions (lower bits).
const (
	// File types (mode & 0xF000)
	ModeTypeMask  = 0xF000
	ModeRegular   = 0x8000 // regular file
	ModeDirectory = 0x4000 // directory
	ModeSymlink   = 0xA000 // symbolic link
	ModeBlock     = 0x6000 // block device
	ModeChar      = 0x2000 // character device
	ModeFifo      = 0x1000 // FIFO
	ModeSocket    = 0xC000 // socket
	ModeObject    = 0x3000 // external storage object (S3, etc.)

	// Permission bits (mode & 0x0FFF)
	PermOwnerRead  = 0x0100
	PermOwnerWrite = 0x0080
	PermOwnerExec  = 0x0040
	PermGroupRead  = 0x0020
	PermGroupWrite = 0x0010
	PermGroupExec  = 0x0008
	PermOtherRead  = 0x0004
	PermOtherWrite = 0x0002
	PermOtherExec  = 0x0001

	// Common permission combinations
	PermOwnerRWX = PermOwnerRead | PermOwnerWrite | PermOwnerExec
	PermGroupRX  = PermGroupRead | PermGroupExec
	PermOtherRX  = PermOtherRead | PermOtherExec
	PermOtherR   = PermOtherRead
	PermOwnerRW  = PermOwnerRead | PermOwnerWrite
)

// Inode flags
const (
	FlagRoot = 1 << iota // Root directory of a filesystem
)

// Inode represents a file system inode (metadata only, no filename).
// Follows Linux inode structure: mode, uid, gid, size, timestamps.
type Inode struct {
	id        string
	systemID  string
	mode      int
	uid       int
	gid       int
	size      int64
	linkCount int
	flags     int
	atime     time.Time
	mtime     time.Time
	ctime     time.Time
	content   []byte
	createdAt time.Time
	updatedAt time.Time
}

// NewInode creates a new Inode.
func NewInode(
	id string,
	systemID string,
	mode int,
	uid int,
	gid int,
	size int64,
	linkCount int,
	flags int,
	atime time.Time,
	mtime time.Time,
	ctime time.Time,
	content []byte,
	createdAt time.Time,
	updatedAt time.Time,
) *Inode {
	return &Inode{
		id:        id,
		systemID:  systemID,
		mode:      mode,
		uid:       uid,
		gid:       gid,
		size:      size,
		linkCount: linkCount,
		flags:     flags,
		atime:     atime,
		mtime:     mtime,
		ctime:     ctime,
		content:   content,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

// Getters
func (i *Inode) ID() string           { return i.id }
func (i *Inode) SystemID() string     { return i.systemID }
func (i *Inode) Mode() int            { return i.mode }
func (i *Inode) UID() int             { return i.uid }
func (i *Inode) GID() int             { return i.gid }
func (i *Inode) Size() int64          { return i.size }
func (i *Inode) LinkCount() int       { return i.linkCount }
func (i *Inode) Flags() int           { return i.flags }
func (i *Inode) Atime() time.Time     { return i.atime }
func (i *Inode) Mtime() time.Time     { return i.mtime }
func (i *Inode) Ctime() time.Time     { return i.ctime }
func (i *Inode) Content() []byte      { return i.content }
func (i *Inode) CreatedAt() time.Time { return i.createdAt }
func (i *Inode) UpdatedAt() time.Time { return i.updatedAt }

// FileType returns the file type portion of the mode.
func (i *Inode) FileType() int {
	return i.mode & ModeTypeMask
}

// Permissions returns the permission portion of the mode.
func (i *Inode) Permissions() int {
	return i.mode & 0x0FFF
}

// IsDir returns true if the inode represents a directory.
func (i *Inode) IsDir() bool {
	return i.FileType() == ModeDirectory
}

// IsRegular returns true if the inode represents a regular file.
func (i *Inode) IsRegular() bool {
	return i.FileType() == ModeRegular
}

// IsSymlink returns true if the inode represents a symbolic link.
func (i *Inode) IsSymlink() bool {
	return i.FileType() == ModeSymlink
}

// IsObject returns true if the inode represents an external storage object.
func (i *Inode) IsObject() bool {
	return i.FileType() == ModeObject
}

// AccessType represents the type of access being requested.
type AccessType int

const (
	AccessRead AccessType = iota
	AccessWrite
	AccessExecute
)

// CheckPerm checks if the given user has the specified access permission.
func (i *Inode) CheckPerm(uid int, gids []int, access AccessType) error {
	if uid == 0 {
		return nil // root bypass
	}

	mode := i.Permissions()

	var perm int
	switch {
	case i.UID() == uid:
		perm = ownerPerm(mode, access)
	case slices.Contains(gids, i.GID()):
		perm = groupPerm(mode, access)
	default:
		perm = otherPerm(mode, access)
	}

	if perm == 0 {
		return errors.Forbidden("permission denied")
	}
	return nil
}

// CheckRead is a convenience method for checking read permission.
func (i *Inode) CheckRead(uid int, gids []int) error {
	return i.CheckPerm(uid, gids, AccessRead)
}

// CheckWrite is a convenience method for checking write permission.
func (i *Inode) CheckWrite(uid int, gids []int) error {
	return i.CheckPerm(uid, gids, AccessWrite)
}

// CheckExecute is a convenience method for checking execute permission.
func (i *Inode) CheckExecute(uid int, gids []int) error {
	return i.CheckPerm(uid, gids, AccessExecute)
}

func ownerPerm(mode int, access AccessType) int {
	switch access {
	case AccessRead:
		return mode & PermOwnerRead
	case AccessWrite:
		return mode & PermOwnerWrite
	case AccessExecute:
		return mode & PermOwnerExec
	}
	return 0
}

func groupPerm(mode int, access AccessType) int {
	switch access {
	case AccessRead:
		return mode & PermGroupRead
	case AccessWrite:
		return mode & PermGroupWrite
	case AccessExecute:
		return mode & PermGroupExec
	}
	return 0
}

func otherPerm(mode int, access AccessType) int {
	switch access {
	case AccessRead:
		return mode & PermOtherRead
	case AccessWrite:
		return mode & PermOtherWrite
	case AccessExecute:
		return mode & PermOtherExec
	}
	return 0
}

// ReadDir reads directory entries from an inode.
// Returns error if the inode is not a directory.
func (i *Inode) ReadDir() ([]content.DirEntry, error) {
	if !i.IsDir() {
		return nil, errors.BadRequest("not a directory")
	}

	var dirContent content.DirContent
	if i.content != nil {
		if err := json.Unmarshal(i.content, &dirContent); err != nil {
			return nil, errors.WrapInternal(err, "failed to parse directory content")
		}
	}

	return dirContent.Entries, nil
}

// IsEmptyDir checks if a directory inode has no user-visible entries.
// Ignores "." and ".." entries. Returns true for nil or unparsable content.
func (i *Inode) IsEmptyDir() bool {
	if !i.IsDir() {
		return false
	}
	if i.content == nil {
		return true
	}
	var dirContent content.DirContent
	if err := json.Unmarshal(i.content, &dirContent); err != nil {
		return true
	}
	for _, e := range dirContent.Entries {
		if e.Name != "." && e.Name != ".." {
			return false
		}
	}
	return true
}

// SymlinkTarget returns the target path of a symlink inode.
func (i *Inode) SymlinkTarget() (string, error) {
	if !i.IsSymlink() {
		return "", errors.BadRequest("not a symlink")
	}
	var symContent content.SymlinkContent
	if err := json.Unmarshal(i.content, &symContent); err != nil {
		return "", errors.WrapInternal(err, "failed to parse symlink content")
	}
	return symContent.Target, nil
}

// ObjectID returns the referenced object ID for object inodes.
func (i *Inode) ObjectID() (string, error) {
	if !i.IsObject() {
		return "", errors.BadRequest("not an object inode")
	}
	var objContent content.ObjectContent
	if err := json.Unmarshal(i.content, &objContent); err != nil {
		return "", errors.WrapInternal(err, "failed to parse object content")
	}
	return objContent.ObjectID, nil
}

// CreateCommand for creating a new inode (service layer).
type CreateCommand struct {
	SystemID string
	Mode     int
	UID      int
	GID      int
	Size     int64
	Flags    int
	Content  []byte
}

// UpdateCommand for updating an inode (service layer).
type UpdateCommand = UpdateParams

// Filter for querying inodes (service layer).
type Filter = QueryFilter

// Filter helpers
func ByID(id string) Filter {
	return Filter{ID: &id}
}

func BySystemID(systemID string) Filter {
	return Filter{SystemID: &systemID}
}

func ByUID(uid int) Filter {
	return Filter{UID: &uid}
}
