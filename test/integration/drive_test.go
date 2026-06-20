package integration

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDrive(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("POST", srv.URL+"/v1/drives", bytes.NewReader([]byte(
		`{"name":"my-drive","storage":{"bucket":"b","region":"us-east-1","accessKey":"a","secretKey":"s"}}`)))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestGetDrive(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("GET", srv.URL+"/v1/drives/d1/root", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestListDrives(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("GET", srv.URL+"/v1/drives", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
