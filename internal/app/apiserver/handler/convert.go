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
	deletedAt := api.OptDateTime{}
	if d.DeletedAt() != nil {
		deletedAt = api.OptDateTime{Value: *d.DeletedAt(), Set: true}
	}
	return &api.Drive{
		ID:          toOptString(d.ID()),
		PublicID:    toOptString(d.PublicID()),
		Name:        toOptString(d.Name()),
		Description: toOptStringPtr(d.Description()),
		OwnerID:     toOptString(d.OwnerID()),
		RootNodeID:  toOptString(rids),
		DeletedAt:   deletedAt,
		CreatedAt:   api.OptDateTime{Value: d.CreatedAt(), Set: true},
		UpdatedAt:   api.OptDateTime{Value: d.UpdatedAt(), Set: true},
	}
}

func toOptString(s string) api.OptString {
	return api.OptString{Value: s, Set: true}
}

func toOptStringPtr(s *string) api.OptString {
	if s == nil {
		return api.OptString{}
	}
	return api.OptString{Value: *s, Set: true}
}

func optBool(b bool) api.OptBool {
	return api.OptBool{Value: b, Set: true}
}
