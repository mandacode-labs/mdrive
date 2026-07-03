package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/app/apiserver"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/handler"
	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

const testUserID = "owner1"

// newTestServer returns an ogen server wired against zero-value
// stubs for fs/drive/upload and an in-memory user repo. Use it
// for tests that just need a handler to dispatch against.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newTestServerWith(t, testSecurity{})
}

// newTestServerWith builds a server with a caller-chosen
// SecurityHandler. realSecurityHandler{} exercises the real
// auth.HandleBearerAuth path so regressions in unauth
// propagation (see /auth/me 500 incident) are caught.
func newTestServerWith(t *testing.T, sec api.SecurityHandler) *httptest.Server {
	t.Helper()
	h := handler.New(
		zeroFS{},
		zeroDrive{owner: testUserID},
		newFakeUserSvc(),
		zeroUpload{},
		permission.NopAuthorizer{},
		"",
	)
	ogenServer, err := api.NewServer(h, sec, api.WithErrorHandler(func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
		apiserver.WriteError(w, err)
	}))
	require.NoError(t, err)
	return httptest.NewServer(apiserver.OpenAPIPassthrough(ogenServer))
}

// newTestServerWithAuthBridge exercises the production middleware
// chain: OpenAPIPassthrough wraps auth.Service.AuthBridge, which
// wraps the ogen server. auth.NewForTest populates the
// anonymous-path set from the embedded OpenAPI spec so /health
// stays 200 even with the real security handler in front of it.
func newTestServerWithAuthBridge(t *testing.T) *httptest.Server {
	t.Helper()
	h := handler.New(
		zeroFS{},
		zeroDrive{owner: testUserID},
		newFakeUserSvc(),
		zeroUpload{},
		permission.NopAuthorizer{},
		"",
	)
	authSvc := auth.NewForTest(newFakeUserSvc())
	ogenServer, err := api.NewServer(h, realSecurityHandler{}, api.WithErrorHandler(func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
		apiserver.WriteError(w, err)
	}))
	require.NoError(t, err)
	chain := authSvc.AuthBridge(ogenServer)
	return httptest.NewServer(apiserver.OpenAPIPassthrough(chain))
}

// testSecurity injects a session for the test user on every
// request. Used by tests that just need a session, not by tests
// of auth itself.
type testSecurity struct{}

func (testSecurity) HandleBearerAuth(ctx context.Context, _ api.OperationName, _ api.BearerAuth) (context.Context, error) {
	return auth.ContextWithSession(ctx, &auth.Session{UserID: testUserID}), nil
}

// realSecurityHandler mirrors the production auth.HandleBearerAuth
// so unauth propagation regressions are caught. /health stays
// anonymous — k8s probes do not carry a session.
type realSecurityHandler struct{}

func (realSecurityHandler) HandleBearerAuth(ctx context.Context, op api.OperationName, _ api.BearerAuth) (context.Context, error) {
	if op == api.HealthOperation {
		return ctx, nil
	}
	if auth.SessionFromContext(ctx) != nil {
		return ctx, nil
	}
	return ctx, errorx.New(errorx.KindUnauthenticated, "auth: no session for bearer token")
}

// newFakeUserSvc returns a *user.Service backed by an in-memory
// fake repo pre-seeded with the test user.
func newFakeUserSvc() *user.Service {
	now := time.Now()
	repo := &userRepoFake{users: map[string]*user.User{
		testUserID: user.NewUser(testUserID, "pub-"+testUserID, "Test User", nil, "google", "test-provider-id", now, now),
	}}
	return user.NewService(repo)
}

type userRepoFake struct {
	users map[string]*user.User
}

var _ user.Repository = (*userRepoFake)(nil)

func (r *userRepoFake) Create(_ context.Context, cmd *user.CreateCommand) (*user.User, error) {
	id := user.GenerateID()
	now := time.Now()
	u := user.NewUser(id, "pub-"+id, cmd.Name, cmd.Email, cmd.Provider, cmd.ProviderID, now, now)
	r.users[u.ID()] = u
	return u, nil
}
func (r *userRepoFake) GetByID(_ context.Context, id string) (*user.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, errorx.New(errorx.KindNotFound, "user: not found")
	}
	return u, nil
}
func (r *userRepoFake) GetByPublicID(_ context.Context, publicID string) (*user.User, error) {
	for _, u := range r.users {
		if u.PublicID() == publicID {
			return u, nil
		}
	}
	return nil, errorx.New(errorx.KindNotFound, "user: not found")
}
func (r *userRepoFake) GetByProviderID(_ context.Context, provider, providerID string) (*user.User, error) {
	for _, u := range r.users {
		if u.Provider() == provider && u.ProviderID() == providerID {
			return u, nil
		}
	}
	return nil, errorx.New(errorx.KindNotFound, "user: not found")
}
func (r *userRepoFake) Exist(_ context.Context, id string) (bool, error) {
	_, ok := r.users[id]
	return ok, nil
}
func (r *userRepoFake) Update(_ context.Context, u *user.User) (*user.User, error) {
	r.users[u.ID()] = u
	return u, nil
}
func (r *userRepoFake) Delete(_ context.Context, id string) error {
	delete(r.users, id)
	return nil
}

// zeroFS is a handler.FSClient stub that returns the zero value
// of every method. Integration tests that need different behaviour
// can satisfy FSClient with their own type.
type zeroFS struct{}

var _ handler.FSClient = zeroFS{}

func (zeroFS) ResolveForPermission(context.Context, string, string) (vfs.PartialResolution, error) {
	return vfs.PartialResolution{}, nil
}
func (zeroFS) Lstat(context.Context, string, string) (vfs.Resolved, error) {
	return vfs.Resolved{}, nil
}
func (zeroFS) GetByID(context.Context, string) (*node.Node, error) { return nodeOrNil(), nil }
func (zeroFS) Mkdir(context.Context, string, string) (*node.Node, error) {
	n, _ := node.NewDirectory()
	return n, nil
}
func (zeroFS) Touch(context.Context, string, string) (*node.Node, error) {
	n, _ := node.NewFile("")
	return n, nil
}
func (zeroFS) Rm(context.Context, string, []string, bool) error           { return nil }
func (zeroFS) Mv(context.Context, string, []string, string, string) error { return nil }
func (zeroFS) Ls(context.Context, string, string) (node.DirContent, error) {
	return node.DirContent{}, nil
}
func (zeroFS) Cat(context.Context, string, string) ([]byte, error) {
	return []byte("hello"), nil
}
func (zeroFS) Write(context.Context, string, string, string) error { return nil }
func (zeroFS) WriteLarge(context.Context, string, string, node.ObjectContent, int64) error {
	return nil
}
func (zeroFS) Stat(context.Context, string, string) (*node.Node, error) {
	n, _ := node.NewFile("")
	return n, nil
}
func (zeroFS) Symlink(context.Context, string, string, string) (*node.Node, error) {
	n, _ := node.NewSymlink("")
	return n, nil
}
func (zeroFS) Hardlink(context.Context, string, string, string) (*node.Node, error) {
	n, _ := node.NewFile("")
	return n, nil
}
func (zeroFS) Mount(context.Context, string, string, string) error { return nil }
func (zeroFS) Unmount(context.Context, string, string) error       { return nil }

// zeroDrive is a handler.DriveClient stub that returns a single
// canned drive owned by the configured owner.
type zeroDrive struct{ owner string }

var _ handler.DriveClient = zeroDrive{}

func (d zeroDrive) Create(context.Context, string, string, string, drive.StorageConfig) (*drive.Drive, uuid.UUID, error) {
	dr := driveOrNil(d.owner)
	return dr, uuid.New(), nil
}
func (d zeroDrive) GetByID(context.Context, string) (*drive.Drive, error) {
	return driveOrNil(d.owner), nil
}
func (d zeroDrive) GetStorage(_ context.Context, driveID string) (*drive.Storage, error) {
	return drive.NewStorage(driveID, "bucket", nil, "us-east-1", "a", "s", false), nil
}
func (d zeroDrive) Update(context.Context, string, string, string) (*drive.Drive, error) {
	return driveOrNil(d.owner), nil
}
func (d zeroDrive) Delete(context.Context, string) error { return nil }
func (d zeroDrive) Restore(context.Context, string) (*drive.Drive, error) {
	return driveOrNil(d.owner), nil
}
func (d zeroDrive) ListByOwner(_ context.Context, actorID string) ([]*drive.Drive, error) {
	return []*drive.Drive{driveOrNil(actorID)}, nil
}
func (d zeroDrive) ListDeletedForAdmin(_ context.Context, _ bool, _ time.Time, _ int) ([]*drive.Drive, error) {
	return []*drive.Drive{driveOrNil(d.owner)}, nil
}

// zeroUpload is a handler.UploadClient stub that returns canned
// presign URLs and a placeholder node on completion.
type zeroUpload struct{}

var _ handler.UploadClient = zeroUpload{}

func (zeroUpload) InitiateUpload(context.Context, string, string, string, *string, *int64, time.Duration) (upload.PresignInfo, error) {
	return upload.PresignInfo{Method: "PUT", URL: "https://s3.example.com/put", Headers: map[string]string{}}, nil
}
func (zeroUpload) CompleteUpload(context.Context, string, string, string, int64, *string) (*node.Node, error) {
	n, _ := node.NewObject(node.ObjectContent{Bucket: "b", Key: "k"}, 0)
	return n, nil
}
func (zeroUpload) PresignDownload(context.Context, string, string, string, time.Duration) (upload.PresignInfo, error) {
	return upload.PresignInfo{Method: "GET", URL: "https://s3.example.com/get", Headers: map[string]string{}}, nil
}

// driveOrNil, nodeOrNil build a populated entity or nil so the
// stubs have something concrete to return without each stub
// repeating the same constructor.
func driveOrNil(owner string) *drive.Drive {
	now := time.Now()
	return drive.NewDrive("d1", "pub-d1", "test-drive", nil, drive.ProviderS3, owner, nil, nil, now, now)
}

func nodeOrNil() *node.Node {
	n, _ := node.NewFile("")
	return n
}
