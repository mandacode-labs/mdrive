package handler

import (
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	apiv1 "github.com/mandacode-labs/mdrive/pkg/apiv1"
)

// --- Conversion helpers ---

func driveToAPI(d *drive.Drive) *apiv1.Drive {
	if d == nil {
		return nil
	}
	rid := d.RootNodeID()
	var rids string
	if rid != nil {
		rids = rid.String()
	}
	return &apiv1.Drive{
		ID:          apistr(d.ID()),
		PublicID:    apistr(d.PublicID()),
		Name:        apistr(d.Name()),
		Description: apistrPtr(d.Description()),
		OwnerID:     apistr(d.OwnerID()),
		RootNodeID:  apistr(rids),
		CreatedAt:   apiv1.OptDateTime{Value: d.CreatedAt(), Set: true},
		UpdatedAt:   apiv1.OptDateTime{Value: d.UpdatedAt(), Set: true},
	}
}

func apistr(s string) apiv1.OptString {
	return apiv1.OptString{Value: s, Set: true}
}

func apistrPtr(s *string) apiv1.OptString {
	if s == nil {
		return apiv1.OptString{}
	}
	return apiv1.OptString{Value: *s, Set: true}
}

func optBool(b bool) apiv1.OptBool {
	return apiv1.OptBool{Value: b, Set: true}
}
