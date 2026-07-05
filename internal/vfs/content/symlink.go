package content

import (
	"encoding/json"

	"github.com/google/uuid"
)

type SymlinkContent struct {
	NodeID uuid.UUID `json:"ino"`
}

func (s *SymlinkContent) Marshal() ([]byte, error) {
	return json.Marshal(s)
}

func NewSymlinkContent(nodeID uuid.UUID) Content {
	return &SymlinkContent{NodeID: nodeID}
}
