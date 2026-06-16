package drive

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewDrive(t *testing.T) {
	desc := "test drive"
	now := time.Now()
	rootID := uuid.New()
	d := NewDrive("01H8X", "01H8Y", "MyDrive", &desc, ProviderS3, "user123", &rootID, now, now)
	if d.ID() != "01H8X" {
		t.Errorf("expected id 01H8X, got %s", d.ID())
	}
	if d.PublicID() != "01H8Y" {
		t.Errorf("expected public id 01H8Y, got %s", d.PublicID())
	}
	if d.Name() != "MyDrive" {
		t.Errorf("expected name MyDrive, got %s", d.Name())
	}
	if d.Description() == nil || *d.Description() != desc {
		t.Errorf("expected description %s, got %v", desc, d.Description())
	}
	if d.Provider() != ProviderS3 {
		t.Errorf("expected provider s3, got %s", d.Provider())
	}
	if d.OwnerID() != "user123" {
		t.Errorf("expected owner_id user123, got %s", d.OwnerID())
	}
	if d.RootNodeID() == nil || *d.RootNodeID() != rootID {
		t.Errorf("expected root_node_id %v, got %v", rootID, d.RootNodeID())
	}
}

func TestSetRootNodeID(t *testing.T) {
	now := time.Now()
	d := NewDrive("id", "pid", "Name", nil, ProviderS3, "user", nil, now, now)
	if d.RootNodeID() != nil {
		t.Error("expected nil root_node_id initially")
	}
	newRoot := uuid.New()
	d.SetRootNodeID(newRoot)
	if d.RootNodeID() == nil || *d.RootNodeID() != newRoot {
		t.Errorf("expected root_node_id %v, got %v", newRoot, d.RootNodeID())
	}
}

func TestNewStorage(t *testing.T) {
	endpoint := "https://s3.example.com"
	s := NewStorage("drive-id", "my-bucket", &endpoint, "us-east-1", "AKIA...", "secret", true)
	if s.DriveID() != "drive-id" {
		t.Errorf("expected drive-id, got %s", s.DriveID())
	}
	if s.Bucket() != "my-bucket" {
		t.Errorf("expected my-bucket, got %s", s.Bucket())
	}
	if s.Endpoint() == nil || *s.Endpoint() != endpoint {
		t.Errorf("expected endpoint %s, got %v", endpoint, s.Endpoint())
	}
	if s.Region() != "us-east-1" {
		t.Errorf("expected us-east-1, got %s", s.Region())
	}
	if s.UsePathStyle() != true {
		t.Error("expected use_path_style true")
	}
}
