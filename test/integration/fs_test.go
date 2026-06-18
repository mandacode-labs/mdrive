package integration

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMkdir(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("POST", srv.URL+"/v1/drives/d1/fs/mkdir", bytes.NewReader([]byte(`{"path":"/foo"}`)))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestTouch(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("POST", srv.URL+"/v1/drives/d1/fs/touch", bytes.NewReader([]byte(`{"path":"/hello.txt"}`)))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestWriteAndCat(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	req := authReq("PUT", srv.URL+"/v1/drives/d1/fs/write", bytes.NewReader([]byte(`{"path":"/data.txt","content":"hello"}`)))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	req2 := authReq("GET", srv.URL+"/v1/drives/d1/fs/cat?path=%2Fdata.txt", nil)
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer func() { _ = resp2.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	got, err := io.ReadAll(resp2.Body)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}

func TestRm(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("DELETE", srv.URL+"/v1/drives/d1/fs", bytes.NewReader([]byte(`{"paths":["/x"]}`)))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestStat(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("GET", srv.URL+"/v1/drives/d1/fs/stat?path=%2Fhello.txt", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestSymlink(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("POST", srv.URL+"/v1/drives/d1/fs/symlink", bytes.NewReader([]byte(`{"target":"/target","linkPath":"/link"}`)))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}
