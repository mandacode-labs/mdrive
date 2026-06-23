package integration

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/app/apiserver"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/handler"
	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/auth/session"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

const testUserID = "owner1"

type stubFS struct{}

func (s *stubFS) ResolveForPermission(context.Context, string, string) (vfs.ResolvedRef, error) {
	return vfs.ResolvedRef{DriveID: "", Path: ""}, nil
}
func (s *stubFS) Mkdir(context.Context, string, string) (*node.Node, error) {
	n, _ := node.NewDirectory()
	return n, nil
}
func (s *stubFS) Touch(context.Context, string, string) (*node.Node, error) {
	n, _ := node.NewFile("")
	return n, nil
}
func (s *stubFS) Rm(context.Context, string, []string, bool) error           { return nil }
func (s *stubFS) Mv(context.Context, string, []string, string, string) error { return nil }
func (s *stubFS) Ls(context.Context, string, string) (node.DirContent, error) {
	return node.DirContent{}, nil
}
func (s *stubFS) Cat(context.Context, string, string) ([]byte, error) {
	return []byte("hello"), nil
}
func (s *stubFS) Write(context.Context, string, string, string) error { return nil }
func (s *stubFS) WriteLarge(context.Context, string, string, node.ObjectContent, int64) error {
	return nil
}
func (s *stubFS) Stat(context.Context, string, string) (*node.Node, error) {
	n, _ := node.NewFile("")
	return n, nil
}
func (s *stubFS) Ln(context.Context, string, string, string, vfs.LinkMode) (*node.Node, error) {
	n, _ := node.NewSymlink("")
	return n, nil
}

var _ handler.FSClient = (*stubFS)(nil)

type stubDrive struct{}

func (s *stubDrive) Create(ctx context.Context, actorID, name, description string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error) {
	rootID := uuid.New()
	d := drive.NewDrive("d1", "pub1", name, nil, drive.ProviderS3, actorID, &rootID, nil, time.Now(), time.Now())
	return d, rootID, nil
}
func (s *stubDrive) Get(ctx context.Context, actorID, id string) (*drive.Drive, error) {
	rootID := uuid.New()
	return drive.NewDrive(id, "pub1", "test", nil, drive.ProviderS3, testUserID, &rootID, nil, time.Now(), time.Now()), nil
}
func (s *stubDrive) GetStorage(ctx context.Context, actorID, driveID string) (*drive.Storage, error) {
	return drive.NewStorage(driveID, "bucket", nil, "us-east-1", "a", "s", false), nil
}
func (s *stubDrive) Update(ctx context.Context, actorID, id string, name, description string) (*drive.Drive, error) {
	var descPtr *string
	if description != "" {
		descPtr = &description
	}
	rootID := uuid.New()
	return drive.NewDrive(id, "pub1", name, descPtr, drive.ProviderS3, testUserID, &rootID, nil, time.Now(), time.Now()), nil
}
func (s *stubDrive) Delete(ctx context.Context, actorID, id string) error { return nil }
func (s *stubDrive) Restore(ctx context.Context, actorID, id string) (*drive.Drive, error) {
	rootID := uuid.New()
	return drive.NewDrive(id, "pub1", "restored", nil, drive.ProviderS3, testUserID, &rootID, nil, time.Now(), time.Now()), nil
}
func (s *stubDrive) ListByOwner(ctx context.Context, actorID string) ([]*drive.Drive, error) {
	rootID := uuid.New()
	d := drive.NewDrive("d1", "pub1", "my-drive", nil, drive.ProviderS3, actorID, &rootID, nil, time.Now(), time.Now())
	return []*drive.Drive{d}, nil
}
func (s *stubDrive) ListDeletedForAdmin(ctx context.Context, isAdmin bool, before time.Time, limit int) ([]*drive.Drive, error) {
	rootID := uuid.New()
	d := drive.NewDrive("d1", "pub1", "deleted-drive", nil, drive.ProviderS3, testUserID, &rootID, nil, time.Now(), time.Now())
	return []*drive.Drive{d}, nil
}

var _ handler.DriveClient = (*stubDrive)(nil)

type stubUpload struct{}

func (s *stubUpload) InitiateUpload(context.Context, string, string, string, *string, *int64, time.Duration) (upload.PresignInfo, error) {
	return upload.PresignInfo{Method: "PUT", URL: "https://s3.example.com/put", Headers: map[string]string{}}, nil
}
func (s *stubUpload) CompleteUpload(ctx context.Context, userID, driveID, uploadID string, contentLength int64, checksum *string) (*node.Node, error) {
	n, _ := node.NewObject(node.ObjectContent{Bucket: "b", Key: "k"}, contentLength)
	return n, nil
}
func (s *stubUpload) PresignDownload(context.Context, string, string, string, time.Duration) (upload.PresignInfo, error) {
	return upload.PresignInfo{Method: "GET", URL: "https://s3.example.com/get", Headers: map[string]string{}}, nil
}

var _ handler.UploadClient = (*stubUpload)(nil)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	userSvc := newFakeUserSvc()
	h := handler.New(&stubFS{}, &stubDrive{}, userSvc, &stubUpload{}, nil, nil, "")
	ogenServer, err := api.NewServer(h, testSecurity{}, api.WithErrorHandler(func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
		apiserver.WriteError(w, err)
	}))
	require.NoError(t, err)
	return httptest.NewServer(ogenServer)
}

// testSecurity injects a session for the test user on every
// request. Mirrors the production SecurityHandler but skips OIDC.
type testSecurity struct{}

func (testSecurity) HandleBearerAuth(ctx context.Context, _ api.OperationName, _ api.BearerAuth) (context.Context, error) {
	sess := &session.Session{UserID: testUserID}
	return auth.ContextWithSession(ctx, sess), nil
}

// newFakeUserSvc returns a *user.Service backed by an in-memory
// fake repository. The integration tests use this for the
// handler's UserClient (which is *user.Service).
func newFakeUserSvc() *user.Service {
	repo := newUserRepoFake()
	now := time.Now()
	// Pre-seed the test user with a known ID so testSecurity can
	// authenticate as them.
	repo.users[testUserID] = user.NewUser(testUserID, "pub-"+testUserID, "Test User", nil, "google", "test-provider-id", now, now)
	return user.NewService(repo)
}

// userRepoFake is a minimal in-memory user.Repository for tests.
type userRepoFake struct {
	users map[string]*user.User
}

func newUserRepoFake() *userRepoFake {
	return &userRepoFake{users: map[string]*user.User{}}
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
		return nil, user.ErrNotFound
	}
	return u, nil
}
func (r *userRepoFake) GetByPublicID(_ context.Context, publicID string) (*user.User, error) {
	for _, u := range r.users {
		if u.PublicID() == publicID {
			return u, nil
		}
	}
	return nil, user.ErrNotFound
}
func (r *userRepoFake) GetByProviderID(_ context.Context, provider, providerID string) (*user.User, error) {
	for _, u := range r.users {
		if u.Provider() == provider && u.ProviderID() == providerID {
			return u, nil
		}
	}
	return nil, user.ErrNotFound
}
func (r *userRepoFake) Update(_ context.Context, u *user.User) (*user.User, error) {
	r.users[u.ID()] = u
	return u, nil
}
func (r *userRepoFake) Delete(_ context.Context, id string) error {
	delete(r.users, id)
	return nil
}

func authReq(method, url string, body io.Reader) *http.Request {
	req, _ := http.NewRequest(method, url, body)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	return req
}
