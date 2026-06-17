package drive

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewDrive(t *testing.T) {
	desc := "test drive"
	now := time.Now()
	rootID := uuid.New()
	d := NewDrive("01H8X", "01H8Y", "MyDrive", &desc, ProviderS3, "user123", &rootID, now, now)

	assert.Equal(t, "01H8X", d.ID())
	assert.Equal(t, "01H8Y", d.PublicID())
	assert.Equal(t, "MyDrive", d.Name())
	assert.NotNil(t, d.Description())
	assert.Equal(t, desc, *d.Description())
	assert.Equal(t, ProviderS3, d.Provider())
	assert.Equal(t, "user123", d.OwnerID())
	assert.NotNil(t, d.RootNodeID())
	assert.Equal(t, rootID, *d.RootNodeID())
}

func TestSetRootNodeID(t *testing.T) {
	now := time.Now()
	d := NewDrive("id", "pid", "Name", nil, ProviderS3, "user", nil, now, now)
	assert.Nil(t, d.RootNodeID())

	newRoot := uuid.New()
	d.SetRootNodeID(newRoot)
	assert.NotNil(t, d.RootNodeID())
	assert.Equal(t, newRoot, *d.RootNodeID())
}
