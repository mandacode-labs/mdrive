package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPISpecIsPublic(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/openapi.json")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(body, &spec), "spec must be valid JSON")
	assert.Equal(t, "3.1.0", spec["openapi"])
	assert.NotEmpty(t, spec["paths"], "spec must include paths")
	assert.NotEmpty(t, spec["components"], "spec must include schemas")
}

func TestOpenAPISpecWorksWithoutSession(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	client := &http.Client{}
	resp, err := client.Get(srv.URL + "/openapi.json")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"/openapi.json must work without a session cookie")
}