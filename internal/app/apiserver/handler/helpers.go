package handler

import (
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

// --- Conversion helpers ---

func driveToAPI(d *drive.Drive) *api.Drive {
	if d == nil {
		return nil
	}
	rid := d.RootNodeID()
	var rids string
	if rid != nil {
		rids = rid.String()
	}
	return &api.Drive{
		ID:          apistr(d.ID()),
		PublicID:    apistr(d.PublicID()),
		Name:        apistr(d.Name()),
		Description: apistrPtr(d.Description()),
		OwnerID:     apistr(d.OwnerID()),
		RootNodeID:  apistr(rids),
		CreatedAt:   api.OptDateTime{Value: d.CreatedAt(), Set: true},
		UpdatedAt:   api.OptDateTime{Value: d.UpdatedAt(), Set: true},
	}
}

func apistr(s string) api.OptString {
	return api.OptString{Value: s, Set: true}
}

func apistrPtr(s *string) api.OptString {
	if s == nil {
		return api.OptString{}
	}
	return api.OptString{Value: *s, Set: true}
}

func optBool(b bool) api.OptBool {
	return api.OptBool{Value: b, Set: true}
}
