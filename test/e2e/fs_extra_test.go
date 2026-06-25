package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func urlEscape(p string) string { return url.QueryEscape(p) }

// createDrive provisions a drive and returns its ID. Mirrors the
// existing TestE2E_FSOperations pattern: create with no parse,
// then list and pick the matching name.
func createDrive(t *testing.T, env *e2eEnv, name, bucket string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"name": name,
		"storage": map[string]string{
			"bucket":    bucket,
			"region":    "us-east-1",
			"accessKey": "a",
			"secretKey": "s",
		},
	})
	req := env.authReq("POST", "/v1/drives", bytes.NewReader(body))
	resp, err := env.apiClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)

	req = env.authReq("GET", "/v1/drives", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var drives []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&drives))
	for _, d := range drives {
		if d["name"] == name {
			return d["id"].(string)
		}
	}
	t.Fatalf("created drive %q not in list", name)
	return ""
}

func TestE2EMv(t *testing.T) {
	env := setupE2E(t)
	driveID := createDrive(t, env, "mv-drive", "mv-bucket")

	mkdir := func(p string) {
		req := env.authReq("POST", "/v1/drives/"+driveID+"/fs/mkdir", bytes.NewReader([]byte(`{"path":"`+p+`"}`)))
		resp, err := env.apiClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	touch := func(p string) {
		req := env.authReq("POST", "/v1/drives/"+driveID+"/fs/touch", bytes.NewReader([]byte(`{"path":"`+p+`"}`)))
		resp, err := env.apiClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	stat := func(p string) int {
		req := env.authReq("GET", "/v1/drives/"+driveID+"/fs/stat?path="+urlEscape(p), nil)
		resp, err := env.apiClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode
	}
	mv := func(sources []string, dest string) int {
		body, _ := json.Marshal(map[string]any{"sources": sources, "destination": dest})
		req := env.authReq("POST", "/v1/drives/"+driveID+"/fs/mv", bytes.NewReader(body))
		resp, err := env.apiClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	mkdir("/src")
	mkdir("/dst")
	touch("/src/a.txt")
	touch("/src/b.txt")

	// Rename within the same directory — this is the case that
	// previously hit ErrRevisionConflict because src and dst parents
	// were loaded as two distinct *Node pointers.
	assert.Equal(t, http.StatusOK, mv([]string{"/src/a.txt"}, "/src/a-renamed.txt"))
	assert.Equal(t, http.StatusNotFound, stat("/src/a.txt"))
	assert.Equal(t, http.StatusOK, stat("/src/a-renamed.txt"))

	// Move two sources into a directory in a single Mv call.
	assert.Equal(t, http.StatusOK, mv([]string{"/src/a-renamed.txt", "/src/b.txt"}, "/dst"))
	assert.Equal(t, http.StatusOK, stat("/dst/a-renamed.txt"))
	assert.Equal(t, http.StatusOK, stat("/dst/b.txt"))
	assert.Equal(t, http.StatusNotFound, stat("/src/a-renamed.txt"))
	assert.Equal(t, http.StatusNotFound, stat("/src/b.txt"))
}

func TestE2ESymlink(t *testing.T) {
	env := setupE2E(t)
	driveID := createDrive(t, env, "symlink-drive", "symlink-bucket")

	mkdir := func(p string) {
		req := env.authReq("POST", "/v1/drives/"+driveID+"/fs/mkdir", bytes.NewReader([]byte(`{"path":"`+p+`"}`)))
		resp, err := env.apiClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	touch := func(p string) {
		req := env.authReq("POST", "/v1/drives/"+driveID+"/fs/touch", bytes.NewReader([]byte(`{"path":"`+p+`"}`)))
		resp, err := env.apiClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	mkdir("/data")
	touch("/data/target.txt")
	write := func(p, content string) {
		req := env.authReq("PUT", "/v1/drives/"+driveID+"/fs/write", bytes.NewReader([]byte(`{"path":"`+p+`","content":"`+content+`"}`)))
		resp, err := env.apiClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	write("/data/target.txt", "target content")

	body, _ := json.Marshal(map[string]any{
		"target":   "/data/target.txt",
		"linkPath": "/link-to-target",
	})
	req := env.authReq("POST", "/v1/drives/"+driveID+"/fs/symlink", bytes.NewReader(body))
	resp, err := env.apiClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Stat follows the symlink (POSIX stat).
	req = env.authReq("GET", "/v1/drives/"+driveID+"/fs/stat?path=%2Flink-to-target", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var statBody struct {
		Type string `json:"type"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&statBody))
	assert.Equal(t, "file", statBody.Type, "stat follows symlinks to the target file")

	// Lstat returns the symlink itself (POSIX lstat).
	req = env.authReq("GET", "/v1/drives/"+driveID+"/fs/lstat?path=%2Flink-to-target", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var lstatBody struct {
		Type string `json:"type"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&lstatBody))
	assert.Equal(t, "symlink", lstatBody.Type, "lstat returns the symlink itself")

	// Readlink returns the symlink's target path (POSIX readlink).
	req = env.authReq("GET", "/v1/drives/"+driveID+"/fs/readlink?path=%2Flink-to-target", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var readlinkBody struct {
		Target string `json:"target"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&readlinkBody))
	assert.Equal(t, "/data/target.txt", readlinkBody.Target)

	// Cat follows the symlink (POSIX cat).
	req = env.authReq("GET", "/v1/drives/"+driveID+"/fs/cat?path=%2Flink-to-target", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "target content", string(got))
}

func TestE2EWriteLarge(t *testing.T) {
	env := setupE2E(t)
	driveID := createDrive(t, env, "writelarge-drive", "writelarge-bucket")

	// WriteLarge just creates the node reference; the body is
	// uploaded via presigned PUT to S3. The e2e can verify the
	// node is created and stat reports the object type.
	body, _ := json.Marshal(map[string]any{
		"path": "/big.bin",
		"size": 1024,
		"object": map[string]any{
			"bucket":      "writelarge-bucket",
			"key":         "drives/" + driveID + "/uploads/test",
			"contentType": "application/octet-stream",
		},
	})
	req := env.authReq("POST", "/v1/drives/"+driveID+"/fs/object", bytes.NewReader(body))
	resp, err := env.apiClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Stat the new node
	req = env.authReq("GET", "/v1/drives/"+driveID+"/fs/stat?path=%2Fbig.bin", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var statBody struct {
		Type string `json:"type"`
		Size int64  `json:"size"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&statBody))
	assert.Equal(t, "object", statBody.Type)
	assert.Equal(t, int64(1024), statBody.Size)

	// Cat returns ObjectNotFound (we didn't actually upload); that's
	// expected — WriteLarge only creates the reference. We don't
	// assert on the cat error here because the e2e fake S3 isn't
	// populated, so just confirm the node is listed.
	req = env.authReq("GET", "/v1/drives/"+driveID+"/fs/ls?path=%2F", nil)
	resp, err = env.apiClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var lsBody struct {
		Entries []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"entries"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&lsBody))
	require.Len(t, lsBody.Entries, 1)
	assert.Equal(t, "big.bin", lsBody.Entries[0].Name)
	assert.Equal(t, "object", lsBody.Entries[0].Type)
}
