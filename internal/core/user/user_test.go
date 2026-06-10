package user_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/core/user"
	userMocks "github.com/mandacode-labs/mdrive/internal/core/user/mocks"
	"github.com/mandacode-labs/mdrive/internal/utils"
)

func TestSystemUser_Struct(t *testing.T) {
	sysUser := user.NewSystemUser(
		1,
		"user-external-123",
		"system-456",
		"testuser",
		1000,
		1000,
	)

	assert.Equal(t, 1, sysUser.ID())
	assert.Equal(t, "user-external-123", sysUser.UserID())
	assert.Equal(t, "system-456", sysUser.SystemID())
	assert.Equal(t, "testuser", sysUser.Username())
	assert.Equal(t, 1000, sysUser.UID())
	assert.Equal(t, 1000, sysUser.GID())
}

func TestUserService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("creates user with auto-assigned UID", func(t *testing.T) {
		userRepo := userMocks.NewSystemUserRepositoryMock(t)
		groupRepo := userMocks.NewSystemGroupRepositoryMock(t)
		svc := user.NewService(userRepo, groupRepo)

		cmd := &user.CreateCommand{
			UserID:   "user-123",
			SystemID: "system-456",
			Username: "testuser",
			UID:      -1, // Auto-assign
		}

		// Mock no existing user
		userRepo.EXPECT().FindOne(mock.Anything, mock.Anything).Return(nil, nil)
		// Mock no existing username
		userRepo.EXPECT().FindOne(mock.Anything, mock.Anything).Return(nil, nil)
		// Mock get next UID
		userRepo.EXPECT().GetNextUID(mock.Anything, "system-456").Return(1000, nil)
		// Mock group creation
		groupRepo.EXPECT().FindOne(mock.Anything, mock.Anything).Return(nil, nil)
		groupRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(
			user.NewSystemGroup(1, "system-456", "testuser", 1000),
			nil,
		)
		// Mock user creation
		expectedUser := user.NewSystemUser(1, "user-123", "system-456", "testuser", 1000, 1000)
		userRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(expectedUser, nil)

		result, err := svc.Create(ctx, cmd)

		require.NoError(t, err)
		assert.Equal(t, 1000, result.UID())
		assert.Equal(t, 1000, result.GID())
	})

	t.Run("creates user with explicit UID", func(t *testing.T) {
		userRepo := userMocks.NewSystemUserRepositoryMock(t)
		groupRepo := userMocks.NewSystemGroupRepositoryMock(t)
		svc := user.NewService(userRepo, groupRepo)

		cmd := &user.CreateCommand{
			UserID:   "user-123",
			SystemID: "system-456",
			Username: "testuser",
			UID:      2000,
		}

		// Mock no existing user
		userRepo.EXPECT().FindOne(mock.Anything, mock.Anything).Return(nil, nil)
		// Mock no existing username
		userRepo.EXPECT().FindOne(mock.Anything, mock.Anything).Return(nil, nil)
		// Mock group creation
		groupRepo.EXPECT().FindOne(mock.Anything, mock.Anything).Return(nil, nil)
		groupRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(
			user.NewSystemGroup(1, "system-456", "testuser", 2000),
			nil,
		)
		// Mock user creation
		expectedUser := user.NewSystemUser(1, "user-123", "system-456", "testuser", 2000, 2000)
		userRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(expectedUser, nil)

		result, err := svc.Create(ctx, cmd)

		require.NoError(t, err)
		assert.Equal(t, 2000, result.UID())
		assert.Equal(t, 2000, result.GID())
	})
}

func TestUserService_ResolveUID(t *testing.T) {
	ctx := context.Background()

	t.Run("resolves UID from context", func(t *testing.T) {
		userRepo := userMocks.NewSystemUserRepositoryMock(t)
		groupRepo := userMocks.NewSystemGroupRepositoryMock(t)
		svc := user.NewService(userRepo, groupRepo)

		// Mock user lookup
		expectedUser := user.NewSystemUser(1, "user-123", "system-456", "testuser", 1000, 1000)
		userRepo.EXPECT().FindOne(mock.Anything, mock.Anything).Return(expectedUser, nil)

		// Create context with user ID using the proper context key
		ctxWithUser := utils.ContextWithUserID(ctx, "user-123")

		uid, err := svc.ResolveUID(ctxWithUser, "system-456")

		require.NoError(t, err)
		assert.Equal(t, 1000, uid)
	})
}

func TestGroupService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("creates group successfully", func(t *testing.T) {
		groupRepo := userMocks.NewSystemGroupRepositoryMock(t)
		svc := user.NewGroupService(groupRepo)

		cmd := &user.GroupCreateCommand{
			SystemID: "system-456",
			Name:     "developers",
			GID:      0, // Auto-assign
		}

		expectedGroup := user.NewSystemGroup(1, "system-456", "developers", 1000)
		groupRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(expectedGroup, nil)

		result, err := svc.Create(ctx, cmd)

		require.NoError(t, err)
		assert.Equal(t, "developers", result.Name())
	})
}

func TestSystemGroup_Struct(t *testing.T) {
	group := user.NewSystemGroup(
		1,
		"system-456",
		"developers",
		1000,
	)

	assert.Equal(t, 1, group.ID())
	assert.Equal(t, "system-456", group.SystemID())
	assert.Equal(t, "developers", group.Name())
	assert.Equal(t, 1000, group.GID())
}
