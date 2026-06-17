package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_FSOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}
	env := setupE2E(t)

	// Create drive first
	createBody := `{"name":"fs-drive","storage":{"bucket":"b","region":"us-east-1","accessKey":"a","secretKey":"s"}}`
	req := env.authReq("POST", "/v1/drives", bytes.NewReader([]byte(createBody)))
	resp, err := env.apiClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// The drive ID format is ULID. We get it from the list.
	req = env.authReq("GET", "/v1/drives", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	var drives []map[string]interface{}
	jsonDecode(resp.Body, &drives)
	resp.Body.Close()
	require.NotEmpty(t, drives)
	driveID := drives[0]["id"].(string)

	// Mkdir
	req = env.authReq("POST", "/v1/drives/"+driveID+"/fs/mkdir", bytes.NewReader([]byte(`{"path":"/docs"}`)))
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Touch
	req = env.authReq("POST", "/v1/drives/"+driveID+"/fs/touch", bytes.NewReader([]byte(`{"path":"/docs/readme.md"}`)))
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Write
	req = env.authReq("PUT", "/v1/drives/"+driveID+"/fs/write", bytes.NewReader([]byte(`{"path":"/docs/readme.md","content":"# Hello E2E"}`)))
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Cat
	catBody := `{"path":"/docs/readme.md"}`
	req = env.authReq("GET", "/v1/drives/"+driveID+"/fs/cat?path=%2Fdocs%2Freadme.md", bytes.NewReader([]byte(catBody)))
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	got, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "# Hello E2E", string(got))

	// Ls
	req = env.authReq("GET", "/v1/drives/"+driveID+"/fs/ls?path=%2Fdocs", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var lsBody struct {
		Entries []struct {
			Name string `json:"name"`
		} `json:"entries"`
	}
	jsonDecode(resp.Body, &lsBody)
	assert.Len(t, lsBody.Entries, 1)
	assert.Equal(t, "readme.md", lsBody.Entries[0].Name)

	// Stat
	req = env.authReq("GET", "/v1/drives/"+driveID+"/fs/stat?path=%2Fdocs%2Freadme.md", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var statBody struct {
		Type string `json:"type"`
		Size int64  `json:"size"`
	}
	jsonDecode(resp.Body, &statBody)
	assert.Equal(t, "file", statBody.Type)
	assert.Equal(t, int64(0), statBody.Size)

	// Rm
	req = env.authReq("DELETE", "/v1/drives/"+driveID+"/fs", bytes.NewReader([]byte(`{"paths":["/docs/readme.md"]}`)))
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Verify removed
	req = env.authReq("GET", "/v1/drives/"+driveID+"/fs/ls?path=%2Fdocs", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	lsBody.Entries = nil
	jsonDecode(resp.Body, &lsBody)
	assert.Len(t, lsBody.Entries, 0)
}

func jsonDecode(r io.Reader, v interface{}) {
	_ = json.NewDecoder(r).Decode(v)
}
