package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/app/apiserver"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/handler"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

const testUserID = "owner1"

type stubFS struct{}

func (s *stubFS) Mkdir(context.Context, string, string, string) (*node.Node, error) {
	n, _ := node.NewDirectory()
	return n, nil
}
func (s *stubFS) Touch(context.Context, string, string, string) (*node.Node, error) {
	n, _ := node.NewFile("")
	return n, nil
}
func (s *stubFS) Rm(context.Context, string, string, []string, bool) error           { return nil }
func (s *stubFS) Mv(context.Context, string, string, []string, string, string) error { return nil }
func (s *stubFS) Ls(context.Context, string, string, string) (node.DirContent, error) {
	return node.DirContent{}, nil
}
func (s *stubFS) Cat(context.Context, string, string, string) ([]byte, error) {
	return []byte("hello"), nil
}
func (s *stubFS) Write(context.Context, string, string, string, string) error { return nil }
func (s *stubFS) WriteLarge(context.Context, string, string, string, node.ObjectContent, int64) error {
	return nil
}
func (s *stubFS) Stat(context.Context, string, string, string) (*node.Node, error) {
	n, _ := node.NewFile("")
	return n, nil
}
func (s *stubFS) Symlink(ctx context.Context, userID, driveID, target, linkPath string) (*node.Node, error) {
	n, _ := node.NewSymlink(target)
	return n, nil
}
func (s *stubFS) InitiateUpload(context.Context, string, string, string, *string, *int64, time.Duration) (vfs.PresignInfo, error) {
	return vfs.PresignInfo{}, nil
}
func (s *stubFS) CompleteUpload(ctx context.Context, userID, driveID, uploadID string, contentLength int64, checksum *string) (*node.Node, error) {
	n, _ := node.NewObject(node.ObjectContent{Bucket: "b", Key: "k"}, contentLength)
	return n, nil
}
func (s *stubFS) PresignDownload(context.Context, string, string, string, time.Duration) (vfs.PresignInfo, error) {
	return vfs.PresignInfo{}, nil
}
func (s *stubFS) CreateDrive(ctx context.Context, actorID, name, description string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error) {
	rootID := uuid.New()
	d := drive.NewDrive("d1", "pub1", name, nil, drive.ProviderS3, actorID, &rootID, time.Now(), time.Now())
	return d, rootID, nil
}
func (s *stubFS) GetDrive(ctx context.Context, id string) (*drive.Drive, error) {
	rootID := uuid.New()
	return drive.NewDrive(id, "pub1", "test", nil, drive.ProviderS3, testUserID, &rootID, time.Now(), time.Now()), nil
}
func (s *stubFS) GetDriveStorage(ctx context.Context, driveID string) (*drive.Storage, error) {
	return drive.NewStorage(driveID, "bucket", nil, "us-east-1", "a", "s", false), nil
}
func (s *stubFS) UpdateDrive(ctx context.Context, id string, name, description *string) (*drive.Drive, error) {
	n := ""
	if name != nil {
		n = *name
	}
	rootID := uuid.New()
	return drive.NewDrive(id, "pub1", n, description, drive.ProviderS3, testUserID, &rootID, time.Now(), time.Now()), nil
}
func (s *stubFS) DeleteDrive(ctx context.Context, id string) error { return nil }
func (s *stubFS) ListUserDrives(ctx context.Context, actorID string) ([]*drive.Drive, error) {
	rootID := uuid.New()
	d := drive.NewDrive("d1", "pub1", "my-drive", nil, drive.ProviderS3, actorID, &rootID, time.Now(), time.Now())
	return []*drive.Drive{d}, nil
}
func (s *stubFS) UpsertUser(ctx context.Context, cmd *user.CreateCommand) (*user.User, error) {
	return user.NewUser("u1", "pub1", cmd.Name, cmd.Email, cmd.Provider, cmd.ProviderID, time.Now(), time.Now()), nil
}
func (s *stubFS) GetUser(ctx context.Context, id string) (*user.User, error) {
	return user.NewUser(id, "pub1", "Tester", nil, "google", "g123", time.Now(), time.Now()), nil
}

var _ handler.FSClient = (*stubFS)(nil)

type noopSecurity struct{}

func (n *noopSecurity) HandleBearerAuth(ctx context.Context, _ api.OperationName, _ api.BearerAuth) (context.Context, error) {
	return ctx, nil
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	h := handler.New(&stubFS{}, func(ctx context.Context) (string, bool) {
		return testUserID, true
	})
	ogenServer, err := api.NewServer(h, &noopSecurity{}, api.WithErrorHandler(func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
		apiserver.WriteError(w, err)
	}))
	require.NoError(t, err)
	return httptest.NewServer(ogenServer)
}

func authReq(method, url string, body io.Reader) *http.Request {
	req, _ := http.NewRequest(method, url, body)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestHealth(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	assert.Equal(t, "ok", body["status"])
}

func TestCreateDrive(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("POST", srv.URL+"/v1/drives", bytes.NewReader([]byte(
		`{"name":"my-drive","storage":{"bucket":"b","region":"us-east-1","accessKey":"a","secretKey":"s"}}`)))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestGetDrive(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("GET", srv.URL+"/v1/drives/d1/root", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestListDrives(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("GET", srv.URL+"/v1/drives", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestMkdir(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("POST", srv.URL+"/v1/drives/d1/fs/mkdir", bytes.NewReader([]byte(`{"path":"/foo"}`)))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestTouch(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("POST", srv.URL+"/v1/drives/d1/fs/touch", bytes.NewReader([]byte(`{"path":"/hello.txt"}`)))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestWriteAndCat(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	req := authReq("PUT", srv.URL+"/v1/drives/d1/fs/write", bytes.NewReader([]byte(`{"path":"/data.txt","content":"hello"}`)))
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	req2 := authReq("GET", srv.URL+"/v1/drives/d1/fs/cat?path=%2Fdata.txt", nil)
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	got, _ := io.ReadAll(resp2.Body)
	assert.Equal(t, "hello", string(got))
}

func TestRm(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("DELETE", srv.URL+"/v1/drives/d1/fs", bytes.NewReader([]byte(`{"paths":["/x"]}`)))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestStat(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("GET", srv.URL+"/v1/drives/d1/fs/stat?path=%2Fhello.txt", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestSymlink(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("POST", srv.URL+"/v1/drives/d1/fs/symlink", bytes.NewReader([]byte(`{"target":"/target","linkPath":"/link"}`)))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestAuthMe_NoAuthConfigured(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	req := authReq("GET", srv.URL+"/auth/me", nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
