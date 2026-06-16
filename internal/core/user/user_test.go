package user

import (
	"testing"
	"time"
)

func TestNewUser(t *testing.T) {
	email := "test@example.com"
	now := time.Now()
	u := NewUser("01H8X", "01H8Y", "Test", &email, "zitadel", "user123", now, now)
	if u.ID() != "01H8X" {
		t.Errorf("expected id 01H8X, got %s", u.ID())
	}
	if u.PublicID() != "01H8Y" {
		t.Errorf("expected public id 01H8Y, got %s", u.PublicID())
	}
	if u.Name() != "Test" {
		t.Errorf("expected name Test, got %s", u.Name())
	}
	if u.Email() == nil || *u.Email() != email {
		t.Errorf("expected email %s, got %v", email, u.Email())
	}
	if u.Provider() != "zitadel" {
		t.Errorf("expected provider zitadel, got %s", u.Provider())
	}
	if u.ProviderID() != "user123" {
		t.Errorf("expected provider_id user123, got %s", u.ProviderID())
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()
	if id1 == id2 {
		t.Error("expected different ULIDs")
	}
	if len(id1) != 26 {
		t.Errorf("expected ULID length 26, got %d", len(id1))
	}
}

func TestNameChanged(t *testing.T) {
	now := time.Now()
	u := NewUser("id", "pid", "Old", nil, "p", "pid", now, now)
	if nameChanged(u, &CreateCommand{Name: "New"}) != true {
		t.Error("expected name changed")
	}
	if nameChanged(u, &CreateCommand{Name: "Old"}) != false {
		t.Error("expected name unchanged")
	}
}

func TestEmailChanged(t *testing.T) {
	now := time.Now()
	email := "old@example.com"
	u := NewUser("id", "pid", "Name", &email, "p", "pid", now, now)

	newEmail := "new@example.com"
	if emailChanged(u, &CreateCommand{Email: &newEmail}) != true {
		t.Error("expected email changed")
	}
	if emailChanged(u, &CreateCommand{Email: &email}) != false {
		t.Error("expected email unchanged")
	}
	if emailChanged(u, &CreateCommand{Email: nil}) != true {
		t.Error("expected email changed (nil vs non-nil)")
	}
	u2 := NewUser("id", "pid", "Name", nil, "p", "pid", now, now)
	if emailChanged(u2, &CreateCommand{Email: nil}) != false {
		t.Error("expected email unchanged (both nil)")
	}
}
