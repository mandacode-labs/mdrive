package e2e

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	permissionMocks "github.com/mandacode-labs/mdrive/internal/permission/mocks"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/valkey-io/valkey-go"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/ent/migrate"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/handler"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/middleware"
	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/entx"
	"github.com/mandacode-labs/mdrive/internal/upload"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type e2eEnv struct {
	pg        *postgres.PostgresContainer
	vk        testcontainers.Container
	pgURL     string
	vkURL     string
	ent       *ent.Client
	vClient   valkey.Client
	userID    string
	server    *httptest.Server
	apiClient *http.Client
	baseURL   string
}

func setupE2E(t *testing.T) *e2eEnv {
	t.Helper()
	ctx := context.Background()

	pg, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("mdrive"),
		postgres.WithUsername("mdrive"),
		postgres.WithPassword("mdrive"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)

	pgURL, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	vkReq := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "valkey/valkey:8-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections"),
		},
		Started: true,
	}
	vk, err := testcontainers.GenericContainer(ctx, vkReq)
	require.NoError(t, err)

	vkHost, err := vk.Host(ctx)
	require.NoError(t, err)
	vkPort, err := vk.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	vClient, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{vkHost + ":" + vkPort.Port()},
	})
	require.NoError(t, err)

	drv, err := entsql.Open("postgres", pgURL)
	require.NoError(t, err)
	entClient := ent.NewClient(ent.Driver(drv))
	err = migrate.Create(ctx, entClient.Schema, migrate.Tables)
	require.NoError(t, err)

	nodeRepo := node.NewRepository(entClient)
	txMgr := entx.NewTxManager(entClient)
	nodeSvc := node.NewService(nodeRepo, txMgr)

	userRepo := user.NewRepository(entClient)
	userSvc := user.NewService(userRepo)
	userEx := userRepo

	u, err := userSvc.UpsertFromOIDC(ctx, &user.CreateCommand{
		Name:       "Test User",
		Provider:   "google",
		ProviderID: "test-user",
	})
	require.NoError(t, err)

	rootDir, err := nodeSvc.CreateDirectory(ctx)
	require.NoError(t, err)

	driveRepo := drive.NewRepository(entClient, nil)
	driveSvc := drive.NewService(driveRepo, userEx, &rootNodeCreator{rootID: rootDir.ID()}, txMgr)

	fs := vfs.NewService(vfs.ServiceConfig{
		NodeClient:      nodeSvc,
		DriveClient:     driveSvc,
		GarbageRecorder: nil,
		TxManager:       txMgr,
	})

	uploadSvcVfs := upload.NewService(upload.Config{
		StorageLookup: driveSvc,
		NodeLifecycle: nodeSvc,
		ObjectStore:   nil,
		Path:          fs,
		TxManager:     txMgr,
	})

	h := handler.New(fs, driveSvc, userSvc, uploadSvcVfs, newAllowAllAuthorizer(t), "",
		handler.WithDefaultStorage(drive.StorageConfig{
			Bucket:       "e2e-bucket",
			Region:       "us-east-1",
			AccessKey:    "a",
			SecretKey:    "s",
			UsePathStyle: false,
		}))

	ogenServer, err := api.NewServer(h, e2eSecurity{userID: u.ID()},
		api.WithMiddleware(middleware.ErrorMiddleware(), middleware.PanicMiddleware()),
	)
	require.NoError(t, err)

	srv := httptest.NewServer(ogenServer)

	env := &e2eEnv{
		pg:        pg,
		vk:        vk,
		pgURL:     pgURL,
		vkURL:     vkHost + ":" + vkPort.Port(),
		ent:       entClient,
		vClient:   vClient,
		userID:    u.ID(),
		server:    srv,
		apiClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:   srv.URL,
	}
	t.Cleanup(func() {
		srv.Close()
		_ = entClient.Close()
		vClient.Close()
		_ = pg.Terminate(ctx)
		_ = vk.Terminate(ctx)
	})
	return env
}

type rootNodeCreator struct {
	rootID uuid.UUID
}

func (r *rootNodeCreator) CreateRootDirectory(ctx context.Context) (uuid.UUID, error) {
	return r.rootID, nil
}

// newAllowAllAuthorizer returns a mock Authorizer that allows
// every Check, returns an empty ListObjects, and no-ops Grant/Revoke.
// Replaces permission.NopAuthorizer (removed) without changing
// e2e semantics. .Maybe() keeps each expectation optional.
func newAllowAllAuthorizer(t *testing.T) *permissionMocks.AuthorizerMock {
	t.Helper()
	a := permissionMocks.NewAuthorizerMock(t)
	a.On("Check", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Maybe()
	a.On("Grant", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	a.On("Revoke", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	a.On("ListObjects", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{}, nil).Maybe()
	return a
}

func (e *e2eEnv) authReq(method, path string, body io.Reader) *http.Request {
	req, _ := http.NewRequest(method, e.baseURL+path, body)
	req.AddCookie(&http.Cookie{Name: "mdrive_session", Value: "e2e-bypass"})
	req.Header.Set("Content-Type", "application/json")
	return req
}

// e2eSecurity injects a session for a fixed user on every request.
// Mirrors the production SecurityHandler but skips the OIDC flow.
type e2eSecurity struct {
	userID string
}

func (e e2eSecurity) HandleCookieAuth(ctx context.Context, _ api.OperationName, _ api.CookieAuth) (context.Context, error) {
	sess := &auth.Session{UserID: e.userID}
	return auth.ContextWithSession(ctx, sess), nil
}
