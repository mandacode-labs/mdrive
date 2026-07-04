package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/app/apiserver/handler"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/middleware"
	apiserverSpec "github.com/mandacode-labs/mdrive/internal/app/apiserver/spec"
	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	permissionMocks "github.com/mandacode-labs/mdrive/internal/permission/mocks"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

const testUserID = "owner1"

// newAllowAllAuthorizer returns a mock Authorizer that allows
// every Check, returns an empty ListObjects, and no-ops Grant/Revoke.
// Replaces permission.NopAuthorizer (removed) without changing
// test semantics. .Maybe() makes each expectation optional so
// tests that never touch the permission layer don't fail with
// "expectation not met".
func newAllowAllAuthorizer(t *testing.T) *permissionMocks.AuthorizerMock {
	t.Helper()
	a := permissionMocks.NewAuthorizerMock(t)
	a.On("Check", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Maybe()
	a.On("Grant", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	a.On("Revoke", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	a.On("ListObjects", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{}, nil).Maybe()
	return a
}

// newTestServer returns an ogen server wired against zero-value
// stubs for fs/drive/upload and an in-memory user repo. Use it
// for tests that just need a handler to dispatch against.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newTestServerWith(t, testSecurity{})
}

// newTestServerWith builds a server with a caller-chosen
// SecurityHandler. The production error/panic middleware is
// always wired in so tests exercise the same error path as prod.
func newTestServerWith(t *testing.T, sec api.SecurityHandler) *httptest.Server {
	t.Helper()
	h := handler.New(
		zeroFS{},
		zeroDrive{owner: testUserID},
		newFakeUserSvc(),
		zeroUpload{},
		newAllowAllAuthorizer(t),
		"",
	)
	ogenServer, err := api.NewServer(h, sec,
		api.WithMiddleware(middleware.ErrorMiddleware(), middleware.PanicMiddleware()),
	)
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle("/openapi.json", apiserverSpecHandler())
	mux.Handle("/", ogenServer)
	return httptest.NewServer(mux)
}

// newTestServerWithAuthBridge exercises the production middleware
// chain: openapi.Handler() on /openapi.json and ogen server on
// everything else. The real SecurityHandler is wired up so /health
// stays anonymous by virtue of OpenAPI's `security: []`.
func newTestServerWithAuthBridge(t *testing.T) *httptest.Server {
	t.Helper()
	h := handler.New(
		zeroFS{},
		zeroDrive{owner: testUserID},
		newFakeUserSvc(),
		zeroUpload{},
		newAllowAllAuthorizer(t),
		"",
	)
	ogenServer, err := api.NewServer(h, realSecurityHandler{},
		api.WithMiddleware(middleware.ErrorMiddleware(), middleware.PanicMiddleware()),
	)
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle("/openapi.json", apiserverSpecHandler())
	mux.Handle("/", ogenServer)
	return httptest.NewServer(mux)
}

// apiserverSpecHandler is a thin wrapper around spec.Handler so
// the test files do not have to import the spec package directly.
// Keeping the import surface narrow here makes it easier to swap
// the spec source in tests later.
func apiserverSpecHandler() http.Handler { return apiserverSpec.Handler() }

// testSecurity injects a session for the test user on every
// request. Used by tests that just need a session, not by tests
// of auth itself.
type testSecurity struct{}

func (testSecurity) HandleCookieAuth(ctx context.Context, _ api.OperationName, _ api.CookieAuth) (context.Context, error) {
	return auth.ContextWithSession(ctx, &auth.Session{UserID: testUserID}), nil
}

// realSecurityHandler mirrors the production auth.HandleCookieAuth
// so unauth propagation regressions are caught. /health stays
// anonymous — k8s probes do not carry a session.
type realSecurityHandler struct{}

func (realSecurityHandler) HandleCookieAuth(ctx context.Context, op api.OperationName, _ api.CookieAuth) (context.Context, error) {
	if op == api.HealthOperation {
		return ctx, nil
	}
	if auth.SessionFromContext(ctx) != nil {
		return ctx, nil
	}
	return ctx, errorx.New(errorx.KindUnauthenticated, "auth: no session cookie")
}

// newFakeUserSvc returns a user.Service backed by an in-memory
// fake repo pre-seeded with the test user.
func newFakeUserSvc() user.Service {
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

// zeroFS is a vfs.Service stub that returns the zero value of
// every method. Integration tests that need different behaviour
// can satisfy vfs.Service with their own type.
type zeroFS struct{}

var _ vfs.Service = zeroFS{}

func (zeroFS) ResolveForPermission(context.Context, string, string) (vfs.PartialResolution, error) {
	return vfs.PartialResolution{}, nil
}
func (zeroFS) Resolve(context.Context, string, string) (vfs.Resolved, error) {
	return vfs.Resolved{}, nil
}
func (zeroFS) Lstat(context.Context, string, string) (vfs.Resolved, error) {
	return vfs.Resolved{}, nil
}
func (zeroFS) GetRootNodeID(context.Context, string) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (zeroFS) ResolveParentNodeID(context.Context, string, string) (uuid.UUID, string, error) {
	return uuid.Nil, "", nil
}
func (zeroFS) ResolveNodeID(context.Context, string, string) (uuid.UUID, error) {
	return uuid.Nil, nil
}
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

// zeroDrive is a drive.Service stub that returns a single
// canned drive owned by the configured owner.
type zeroDrive struct{ owner string }

var _ drive.Service = zeroDrive{}

func (d zeroDrive) Create(context.Context, string, string, string, drive.StorageConfig) (*drive.Drive, uuid.UUID, error) {
	dr := driveOrNil(d.owner)
	return dr, uuid.New(), nil
}
func (d zeroDrive) GetByID(context.Context, string) (*drive.Drive, error) {
	return driveOrNil(d.owner), nil
}
func (d zeroDrive) GetByPublicID(context.Context, string) (*drive.Drive, error) {
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
func (d zeroDrive) Purge(context.Context, string) error { return nil }
func (d zeroDrive) ListByOwner(_ context.Context, actorID string) ([]*drive.Drive, error) {
	return []*drive.Drive{driveOrNil(actorID)}, nil
}
func (d zeroDrive) ListDeletedByOwner(_ context.Context, actorID string) ([]*drive.Drive, error) {
	return []*drive.Drive{driveOrNil(actorID)}, nil
}
func (d zeroDrive) ListDeletedForAdmin(_ context.Context, _ bool, _ time.Time, _ int) ([]*drive.Drive, error) {
	return []*drive.Drive{driveOrNil(d.owner)}, nil
}
func (d zeroDrive) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// zeroUpload is an upload.Service stub that returns canned
// presign URLs and a placeholder node on completion.
type zeroUpload struct{}

var _ upload.Service = zeroUpload{}

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
func (zeroUpload) DeleteObject(context.Context, string, string) error { return nil }

// driveOrNil builds a populated drive so the stubs have something
// concrete to return without each stub repeating the same constructor.
func driveOrNil(owner string) *drive.Drive {
	now := time.Now()
	return drive.NewDrive("d1", "pub-d1", "test-drive", nil, drive.ProviderS3, owner, nil, nil, now, now)
}
