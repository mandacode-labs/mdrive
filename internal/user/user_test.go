package user_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mandacode-labs/mdrive/internal/user"
	userMocks "github.com/mandacode-labs/mdrive/internal/user/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestIsValidProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		expected bool
	}{
		{
			name:     "keycloak provider is valid",
			provider: user.ProviderKeycloak,
			expected: true,
		},
		{
			name:     "google provider is valid",
			provider: user.ProviderGoogle,
			expected: true,
		},
		{
			name:     "empty provider is invalid",
			provider: "",
			expected: false,
		},
		{
			name:     "unknown provider is invalid",
			provider: "unknown",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := user.IsValidProvider(tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUser_Struct(t *testing.T) {
	now := time.Now()
	u := user.NewUser(
		"user-123",
		"testuser",
		user.ProviderKeycloak,
		"keycloak-123",
		now,
		now,
		now,
	)

	assert.Equal(t, "user-123", u.ID())
	assert.Equal(t, "testuser", u.Username())
	assert.Equal(t, user.ProviderKeycloak, u.Provider())
	assert.Equal(t, "keycloak-123", u.ProviderID())
}

func TestService_Get(t *testing.T) {
	ctx := context.Background()

	t.Run("returns user when found", func(t *testing.T) {
		userRepo := userMocks.NewUserRepositoryMock(t)
		svc := user.NewService(userRepo)

		expectedUser := user.NewUser(
			"user-123",
			"testuser",
			user.ProviderKeycloak,
			"keycloak-123",
			time.Now(),
			time.Now(),
			time.Now(),
		)
		userRepo.EXPECT().GetByProvider(mock.Anything, user.ProviderKeycloak, "keycloak-123").Return(expectedUser, nil)

		result, err := svc.Get(ctx, user.ProviderKeycloak, "keycloak-123")

		assert.NoError(t, err)
		assert.Equal(t, expectedUser, result)
	})

	t.Run("returns error when user not found", func(t *testing.T) {
		userRepo := userMocks.NewUserRepositoryMock(t)
		svc := user.NewService(userRepo)

		userRepo.EXPECT().GetByProvider(mock.Anything, user.ProviderKeycloak, "keycloak-123").Return(nil, nil)

		result, err := svc.Get(ctx, user.ProviderKeycloak, "keycloak-123")

		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error when repository fails", func(t *testing.T) {
		userRepo := userMocks.NewUserRepositoryMock(t)
		svc := user.NewService(userRepo)

		userRepo.EXPECT().GetByProvider(mock.Anything, user.ProviderKeycloak, "keycloak-123").Return(nil, errors.New("db error"))

		result, err := svc.Get(ctx, user.ProviderKeycloak, "keycloak-123")

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestService_FindOrCreateByOIDC(t *testing.T) {
	ctx := context.Background()

	t.Run("returns existing user", func(t *testing.T) {
		userRepo := userMocks.NewUserRepositoryMock(t)
		svc := user.NewService(userRepo)

		existingUser := user.NewUser(
			"user-123",
			"testuser",
			user.ProviderKeycloak,
			"subject-123",
			time.Now(),
			time.Now(),
			time.Now(),
		)
		userRepo.EXPECT().GetByProvider(mock.Anything, user.ProviderKeycloak, "subject-123").Return(existingUser, nil)

		userID, err := svc.FindOrCreateByOIDC(ctx, user.ProviderKeycloak, "subject-123", "testuser")

		assert.NoError(t, err)
		assert.Equal(t, "user-123", userID)
	})

	t.Run("creates new user when not found", func(t *testing.T) {
		userRepo := userMocks.NewUserRepositoryMock(t)
		svc := user.NewService(userRepo)

		userRepo.EXPECT().GetByProvider(mock.Anything, user.ProviderKeycloak, "subject-123").Return(nil, nil)
		userRepo.EXPECT().ExistsByProvider(mock.Anything, user.ProviderKeycloak, "subject-123").Return(false, nil)
		newUser := user.NewUser(
			"user-456",
			"testuser",
			user.ProviderKeycloak,
			"subject-123",
			time.Now(),
			time.Now(),
			time.Now(),
		)
		userRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(newUser, nil)

		userID, err := svc.FindOrCreateByOIDC(ctx, user.ProviderKeycloak, "subject-123", "testuser")

		assert.NoError(t, err)
		assert.Equal(t, "user-456", userID)
	})
}
