package user

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewUser(t *testing.T) {
	email := "test@example.com"
	now := time.Now()
	u := NewUser("01H8X", "01H8Y", "Test", &email, "keycloak", "user123", now, now)

	assert.Equal(t, "01H8X", u.ID())
	assert.Equal(t, "01H8Y", u.PublicID())
	assert.Equal(t, "Test", u.Name())
	assert.NotNil(t, u.Email())
	assert.Equal(t, email, *u.Email())
	assert.Equal(t, "keycloak", u.Provider())
	assert.Equal(t, "user123", u.ProviderID())
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()
	assert.NotEqual(t, id1, id2)
	assert.Len(t, id1, 26)
}
