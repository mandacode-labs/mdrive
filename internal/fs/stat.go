package fs

import (
	"time"

	"github.com/google/uuid"
)

// Stat is the user-visible inode metadata. Mirrors POSIX
// struct stat.
type Stat struct {
	InodeID  uuid.UUID
	Kind     NodeKind
	Size     int64
	NLink    uint32
	ATime    time.Time
	MTime    time.Time
	CTime    time.Time
	BTime    time.Time
	Flags    Flags
	Revision Revision
}

// NodeToStat converts a Node into the public Stat DTO.
func NodeToStat(n *Node) Stat {
	if n == nil {
		return Stat{}
	}
	return Stat{
		InodeID:  n.ID(),
		Kind:     n.Kind(),
		Size:     n.Size(),
		NLink:    n.NLink(),
		ATime:    n.ATime(),
		MTime:    n.MTime(),
		CTime:    n.CTime(),
		BTime:    n.BTime(),
		Flags:    n.Flags(),
		Revision: n.Revision(),
	}
}
