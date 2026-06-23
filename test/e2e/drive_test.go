package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/pkg/api"
)

func TestE2E_DriveLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}
	env := setupE2E(t)

	// Create drive
	body := `{"name":"e2e-drive","storage":{"bucket":"e2e-bucket","region":"us-east-1","accessKey":"a","secretKey":"s"}}`
	req := env.authReq("POST", "/v1/drives", bytes.NewReader([]byte(body)))
	resp, err := env.apiClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var created api.Drive
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	driveID := created.ID.Value
	require.NotEmpty(t, driveID)

	// Get drive
	req = env.authReq("GET", "/v1/drives/"+driveID+"/root", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// List drives
	req = env.authReq("GET", "/v1/drives", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var drives []api.Drive
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&drives))
	assert.Len(t, drives, 1)

	// Update drive
	updateBody := `{"name":"e2e-drive-updated"}`
	req = env.authReq("PUT", "/v1/drives/"+driveID+"/root", bytes.NewReader([]byte(updateBody)))
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var updated api.Drive
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&updated))
	assert.Equal(t, "e2e-drive-updated", updated.Name.Value)

	// Delete drive
	req = env.authReq("DELETE", "/v1/drives/"+driveID+"/root", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		t.Logf("delete response: %s", body)
	}

	// Verify deleted (soft-delete: GET still returns the drive but with deletedAt)
	req = env.authReq("GET", "/v1/drives/"+driveID+"/root", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify it no longer appears in the drive list
	req = env.authReq("GET", "/v1/drives", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	var listAfter []api.Drive
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&listAfter))
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, listAfter)
}
