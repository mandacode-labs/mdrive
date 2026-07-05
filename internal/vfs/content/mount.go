package content

import "encoding/json"

type MountContent struct {
	DriveID string `json:"drv"`
}

func (m *MountContent) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

func NewMountContent(driveID string) Content {
	return &MountContent{DriveID: driveID}
}
