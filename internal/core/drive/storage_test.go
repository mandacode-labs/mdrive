package drive

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStorage(t *testing.T) {
	endpoint := "https://s3.example.com"
	s := NewStorage("drive-id", "my-bucket", &endpoint, "us-east-1", "AKIA...", "secret", true, "wrapped-dek")

	assert.Equal(t, "drive-id", s.DriveID())
	assert.Equal(t, "my-bucket", s.Bucket())
	assert.NotNil(t, s.Endpoint())
	assert.Equal(t, endpoint, *s.Endpoint())
	assert.Equal(t, "us-east-1", s.Region())
	assert.True(t, s.UsePathStyle())
}
