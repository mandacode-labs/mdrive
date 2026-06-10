//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/mandacode-labs/retrowin-go/ent"
	entsystem "github.com/mandacode-labs/retrowin-go/ent/system"
	entusersystem "github.com/mandacode-labs/retrowin-go/ent/usersystem"
	"github.com/mandacode-labs/retrowin-go/internal/application/storage"
	"github.com/mandacode-labs/retrowin-go/internal/application/vfs"
	"github.com/mandacode-labs/retrowin-go/internal/config"
	"github.com/mandacode-labs/retrowin-go/internal/core/inode"
	inoderepo "github.com/mandacode-labs/retrowin-go/internal/core/inode/repository"
	"github.com/mandacode-labs/retrowin-go/internal/core/object"
	objectrepo "github.com/mandacode-labs/retrowin-go/internal/core/object/repository"
	"github.com/mandacode-labs/retrowin-go/internal/core/object/s3"
	"github.com/mandacode-labs/retrowin-go/internal/core/user"
	"github.com/mandacode-labs/retrowin-go/internal/logging"
	"github.com/mandacode-labs/retrowin-go/internal/service/sysinit"
	domainsystem "github.com/mandacode-labs/retrowin-go/internal/system"
	systemrepo "github.com/mandacode-labs/retrowin-go/internal/system/repository"
	"github.com/mandacode-labs/retrowin-go/internal/utils"
)

// Shared containers for integration tests — started once via sync.Once.
var (
	sharedOnce      sync.Once
	sharedErr       error
	sharedPg        *postgres.PostgresContainer
	sharedMinio     testcontainers.Container
	sharedPgHost    string
	sharedPgPort    int
	sharedMinioAddr string
)

func startSharedContainers(ctx context.Context) error {
	sharedOnce.Do(func() {
		pgContainer, err := postgres.Run(ctx, "postgres:17-alpine",
			postgres.WithDatabase("postgres"),
			postgres.WithUsername("test"),
			postgres.WithPassword("test"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(120*time.Second),
			),
		)
		if err != nil {
			sharedErr = fmt.Errorf("failed to start postgres: %w", err)
			return
		}
		sharedPg = pgContainer

		pgHost, err := pgContainer.Host(ctx)
		if err != nil {
			sharedErr = fmt.Errorf("failed to get postgres host: %w", err)
			return
		}
		pgPort, err := pgContainer.MappedPort(ctx, "5432")
		if err != nil {
			sharedErr = fmt.Errorf("failed to get postgres port: %w", err)
			return
		}
		sharedPgHost = pgHost
		sharedPgPort = int(pgPort.Num())

		minioContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "minio/minio:latest",
				ExposedPorts: []string{"9000/tcp"},
				Env: map[string]string{
					"MINIO_ROOT_USER":     "minioadmin",
					"MINIO_ROOT_PASSWORD": "minioadmin",
				},
				Cmd:        []string{"server", "/data"},
				WaitingFor: wait.ForExposedPort().WithStartupTimeout(120 * time.Second),
			},
			Started: true,
		})
		if err != nil {
			sharedErr = fmt.Errorf("failed to start minio: %w", err)
			return
		}
		sharedMinio = minioContainer

		mnHost, err := minioContainer.Host(ctx)
		if err != nil {
			sharedErr = fmt.Errorf("failed to get minio host: %w", err)
			return
		}
		mnPort, err := minioContainer.MappedPort(ctx, "9000")
		if err != nil {
			sharedErr = fmt.Errorf("failed to get minio port: %w", err)
			return
		}
		sharedMinioAddr = fmt.Sprintf("%s:%s", mnHost, mnPort.Port())

		logger := logging.NewLogger("test", "info")
		logger.Info().
			Str("pg", fmt.Sprintf("%s:%d", sharedPgHost, sharedPgPort)).
			Str("minio", sharedMinioAddr).
			Msg("integration test containers started")
	})
	return sharedErr
}

func stopSharedContainers(ctx context.Context) {
	if sharedPg != nil {
		_ = testcontainers.TerminateContainer(sharedPg)
	}
	if sharedMinio != nil {
		_ = testcontainers.TerminateContainer(sharedMinio)
	}
}

// Suite provides an isolated integration test environment.
type Suite struct {
	EntClient     *ent.Client
	DB            *sql.DB
	ObjectSvc     *object.Service
	ObjectStorage object.Storage
	StorageSvc    storage.StorageService
	FsSvc         vfs.VFS
	InodeSvc      *inode.Service
	UserSvc       *user.Service
	SystemSvc     domainsystem.SystemService
	InitSvc       sysinit.InitService
	MinioClient   *minio.Client
	dbName        string
	bucketName    string
}

// NewSuite creates a new integration test suite.
func NewSuite(t *testing.T) *Suite {
	return &Suite{}
}

// Start sets up the test environment.
func (s *Suite) Start(ctx context.Context) error {
	if err := startSharedContainers(ctx); err != nil {
		return fmt.Errorf("failed to start shared containers: %w", err)
	}

	s.dbName = "retrowin_int_test_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	adminDSN := fmt.Sprintf("host=%s port=%d user=test password=test dbname=postgres sslmode=disable",
		sharedPgHost, sharedPgPort)

	adminDB, err := sql.Open("postgres", adminDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to admin database: %w", err)
	}
	if err = adminDB.PingContext(ctx); err != nil {
		adminDB.Close()
		return fmt.Errorf("failed to ping admin database: %w", err)
	}
	_, err = adminDB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", s.dbName))
	adminDB.Close()
	if err != nil {
		return fmt.Errorf("failed to create test database: %w", err)
	}

	testDSN := fmt.Sprintf("host=%s port=%d user=test password=test dbname=%s sslmode=disable",
		sharedPgHost, sharedPgPort, s.dbName)
	s.DB, err = sql.Open("postgres", testDSN)
	if err != nil {
		return fmt.Errorf("failed to open test database: %w", err)
	}

	drv := entsql.OpenDB(dialect.Postgres, s.DB)
	s.EntClient = ent.NewClient(ent.Driver(drv))

	if err := s.EntClient.Schema.Create(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	s.bucketName = "int-test-bucket-" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	minioClient, err := minio.New(sharedMinioAddr, &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		return fmt.Errorf("failed to create minio client: %w", err)
	}
	s.MinioClient = minioClient

	if err := minioClient.MakeBucket(ctx, s.bucketName, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("failed to create minio bucket: %w", err)
	}

	storageCfg := &config.StorageConfig{
		Provider:  "s3",
		Region:    "us-east-1",
		Endpoint:  "http://" + sharedMinioAddr,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    s.bucketName,
	}
	objStorage, err := s3.New(storageCfg)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	objRepo := objectrepo.NewRepository(s.EntClient)
	s.ObjectSvc = object.NewService(objRepo, objStorage)
	s.ObjectStorage = objStorage

	inodeRepo := inoderepo.NewRepository(s.EntClient)
	s.InodeSvc = inode.NewService(inodeRepo)

	s.UserSvc = user.NewService(
		systemrepo.NewSystemUserRepository(s.EntClient),
		systemrepo.NewSystemGroupRepository(s.EntClient),
	)

	locker := vfs.NewLocker()
	s.FsSvc = vfs.NewService(s.EntClient, s.InodeSvc, s.ObjectSvc, s.ObjectStorage, s.UserSvc, locker)

	storageSvc := storage.NewService(s.FsSvc, s.ObjectSvc)
	s.StorageSvc = storageSvc

	sysRepo := systemrepo.NewRepository(s.EntClient)
	groupRepo := systemrepo.NewSystemGroupRepository(s.EntClient)
	groupSvc := user.NewGroupService(groupRepo)
	s.SystemSvc = domainsystem.NewService(sysRepo, s.InodeSvc, s.ObjectSvc, s.UserSvc, groupSvc)

	s.InitSvc = sysinit.NewService(s.SystemSvc, s.UserSvc, s.FsSvc, s.InodeSvc)

	return nil
}

// Stop cleans up per-test resources.
func (s *Suite) Stop(ctx context.Context) error {
	if s.EntClient != nil {
		_ = s.EntClient.Close()
	}
	if s.DB != nil {
		_ = s.DB.Close()
	}

	if s.dbName != "" {
		adminDSN := fmt.Sprintf("host=%s port=%d user=test password=test dbname=postgres sslmode=disable",
			sharedPgHost, sharedPgPort)
		adminDB, err := sql.Open("postgres", adminDSN)
		if err == nil {
			_, _ = adminDB.ExecContext(ctx, fmt.Sprintf(
				"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s'", s.dbName))
			_, _ = adminDB.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", s.dbName))
			_ = adminDB.Close()
		}
	}

	if s.bucketName != "" && s.MinioClient != nil {
		objectsCh := make(chan minio.ObjectInfo)
		go func() {
			defer close(objectsCh)
			for object := range s.MinioClient.ListObjects(ctx, s.bucketName, minio.ListObjectsOptions{}) {
				if object.Err == nil {
					objectsCh <- object
				}
			}
		}()
		_ = s.MinioClient.RemoveObjects(ctx, s.bucketName, objectsCh, minio.RemoveObjectsOptions{})
		_ = s.MinioClient.RemoveBucket(ctx, s.bucketName)
	}

	return nil
}

// CreateTestUser creates a user directly in the database for testing.
func (s *Suite) CreateTestUser(ctx context.Context, provider, providerID, username string) (*ent.User, error) {
	return s.EntClient.User.Create().
		SetID(uuid.New().String()).
		SetProvider(provider).
		SetProviderID(providerID).
		SetUsername(username).
		Save(ctx)
}

// CreateTestSystem creates a system directly in the database for testing.
func (s *Suite) CreateTestSystem(ctx context.Context, name string) (*ent.System, error) {
	return s.EntClient.System.Create().
		SetID(uuid.New().String()).
		SetName(name).
		SetStatus(entsystem.StatusActive).
		Save(ctx)
}

// CreateTestSystemUser creates a system user mapping.
func (s *Suite) CreateTestSystemUser(ctx context.Context, systemID, userID, username string, uid, gid int) (*ent.UserSystem, error) {
	return s.EntClient.UserSystem.Create().
		SetSystemID(systemID).
		SetUserID(userID).
		SetUsername(username).
		SetUID(uid).
		SetGid(gid).
		Save(ctx)
}

// CreateTestSystemGroup creates a system group.
func (s *Suite) CreateTestSystemGroup(ctx context.Context, systemID, name string, gid int) (*ent.SystemGroup, error) {
	return s.EntClient.SystemGroup.Create().
		SetSystemID(systemID).
		SetName(name).
		SetGid(gid).
		Save(ctx)
}

// BucketName returns the MinIO bucket name used for this test.
func (s *Suite) BucketName() string {
	return s.bucketName
}

// AuthenticatedContext returns a context with the given user ID set for auth.
func (s *Suite) AuthenticatedContext(ctx context.Context, userID string) context.Context {
	return utils.ContextWithUserID(ctx, userID)
}

// SetupFullEnvironment creates a user and initializes a system with root/home directories.
func (s *Suite) SetupFullEnvironment(ctx context.Context, username string) (*ent.User, *ent.System, *ent.UserSystem, error) {
	u, err := s.CreateTestUser(ctx, "keycloak", fmt.Sprintf("test-%s", username), username)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create user: %w", err)
	}

	initResult, err := s.InitSvc.InitSystem(ctx, &sysinit.InitSystemCommand{
		Name:         fmt.Sprintf("%s-system", username),
		RootUserID:   u.ID,
		InitialUsers: []sysinit.InitialUser{},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to initialize system: %w", err)
	}

	sys := s.EntClient.System.GetX(ctx, initResult.System.ID())

	// Fetch the system user created by InitSystem
	su, err := s.EntClient.UserSystem.Query().
		Where(entusersystem.SystemIDEQ(sys.ID)).
		Where(entusersystem.UserIDEQ(u.ID)).
		Only(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to find system user: %w", err)
	}

	return u, sys, su, nil
}
