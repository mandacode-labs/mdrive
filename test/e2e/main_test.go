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
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/valkey-io/valkey-go"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/handler"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type e2eEnv struct {
	pg    *postgres.PostgresContainer
	vk    testcontainers.Container
	pgURL string
	vkURL string
	ent   *ent.Client
	vClient valkey.Client
	server  *httptest.Server
	apiClient *http.Client
	baseURL   string
}

func setupE2E(t *testing.T) *e2eEnv {
	t.Helper()
	ctx := context.Background()

	// Postgres
	pg, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("mdrive"),
		postgres.WithUsername("mdrive"),
		postgres.WithPassword("mdrive"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)

	pgURL, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Valkey
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
	vkPort, err := vk.MappedPort(ctx, "6379")
	require.NoError(t, err)
	vkURL := vkHost + ":" + vkPort.Port()

	vClient, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{vkURL},
	})
	require.NoError(t, err)

	// Ent client + migration
	drv, err := entsql.Open("postgres", pgURL)
	require.NoError(t, err)
	entClient := ent.NewClient(ent.Driver(drv))
	err = entClient.Schema.Create(ctx)
	require.NoError(t, err)

	// Core services
	nodeRepo := node.NewEntRepository(entClient)
	nodeSvc := node.NewService(nodeRepo)

	userRepo := user.NewRepository(entClient)
	userSvc := user.NewService(userRepo)
	userEx := user.NewExisterAdapter(userRepo)

	rootDir, err := nodeSvc.CreateDirectory(ctx)
	require.NoError(t, err)
	rootCreator := &rootNodeCreator{rootID: rootDir.ID()}

	driveRepo := drive.NewRepository(entClient, nil)
	driveSvc := drive.NewService(driveRepo, userEx, rootCreator)

	fs := vfs.NewService(nodeSvc, driveSvc, userSvc, nil, nil, nil, nil)

	// Handler + ogen server
	h := handler.New(fs, func(ctx context.Context) (string, bool) {
		return "default", true
	})

	ogenServer, err := api.NewServer(h, &noopSecurity{}, api.WithErrorHandler(
		func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
			apiserver.WriteError(w, err)
		},
	))
	require.NoError(t, err)

	srv := httptest.NewServer(ogenServer)

	env := &e2eEnv{
		pg:       pg,
		vk:       vk,
		pgURL:    pgURL,
		vkURL:    vkURL,
		ent:      entClient,
		vClient:  vClient,
		server:   srv,
		apiClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:  srv.URL,
	}
	t.Cleanup(func() {
		srv.Close()
		entClient.Close()
		vClient.Close()
		pg.Terminate(ctx)
		vk.Terminate(ctx)
	})
	return env
}

type rootNodeCreator struct {
	rootID uuid.UUID
}

func (r *rootNodeCreator) NewRootDirectory(ctx context.Context) (uuid.UUID, error) {
	return r.rootID, nil
}

type noopSecurity struct{}

func (n *noopSecurity) HandleBearerAuth(ctx context.Context, _ api.OperationName, _ api.BearerAuth) (context.Context, error) {
	return ctx, nil
}

func (e *e2eEnv) authReq(method, path string, body io.Reader) *http.Request {
	req, _ := http.NewRequest(method, e.baseURL+path, body)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	return req
}
