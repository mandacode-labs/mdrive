package e2e

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUser_Get tests the user info endpoint
func TestUser_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	err = suite.StartServer(ctx)
	require.NoError(t, err, "Failed to start server")

	t.Run("returns current user info", func(t *testing.T) {
		// Create and login user
		user, err := suite.SetupAuthenticatedUser(ctx, "testuser")
		require.NoError(t, err, "Failed to setup authenticated user")

		resp, err := suite.Get("/user")
		require.NoError(t, err, "Failed to get user info")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusOK, resp.StatusCode,
			"Expected 200 OK, got %d", resp.StatusCode)

		var result map[string]any
		err = suite.ReadJSON(resp, &result)
		require.NoError(t, err, "Failed to read response JSON")

		// Response is wrapped in "user" object
		userData, ok := result["user"].(map[string]any)
		require.True(t, ok, "Response should contain user object")

		assert.Equal(t, user.ID, userData["id"], "User ID should match")
		assert.Equal(t, "keycloak", userData["provider"], "Provider should match")
		assert.Equal(t, "test-testuser", userData["providerId"], "Provider ID should match")
	})

	t.Run("returns 401 without session", func(t *testing.T) {
		suite.ClearCookies()

		resp, err := suite.Get("/user")
		require.NoError(t, err, "Failed to make request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"Expected 401 Unauthorized, got %d", resp.StatusCode)
	})
}

// TestSystemUser_Create tests creating a system user
func TestSystemUser_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	err = suite.StartServer(ctx)
	require.NoError(t, err, "Failed to start server")

	// Setup user and system via API (for proper filesystem initialization)
	env, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	t.Run("creates user with auto-assigned UID", func(t *testing.T) {
		// Create a real user in the database first
		newUser, err := suite.CreateTestUser(ctx, "test-provider", "auto-user-provider-id", "auto-user")
		require.NoError(t, err, "Failed to create test user")

		req := map[string]any{
			"userId":   newUser.ID,
			"username": "newuser",
		}

		resp, err := suite.Post("/systems/"+systemID+"/users", req)
		require.NoError(t, err, "Failed to create system user")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusCreated, resp.StatusCode, "Expected 201 Created")

		var result map[string]any
		err = suite.ReadJSON(resp, &result)
		require.NoError(t, err, "Failed to read response JSON")

		// Response is wrapped in "user" object
		sysUser, ok := result["user"].(map[string]any)
		require.True(t, ok, "Response should contain user object")

		// UID should be auto-assigned (starting from 1000)
		uid, ok := sysUser["uid"].(float64)
		require.True(t, ok, "UID should be a number")
		assert.GreaterOrEqual(t, int(uid), 1000, "Auto-assigned UID should start from 1000")

		// GID should equal UID (private group)
		gid, ok := sysUser["gid"].(float64)
		require.True(t, ok, "GID should be a number")
		assert.Equal(t, uid, gid, "GID should equal UID for private group")
	})

	t.Run("creates user with explicit UID", func(t *testing.T) {
		// Create a real user in the database first
		newUser, err := suite.CreateTestUser(ctx, "test-provider", "explicit-user-provider-id", "explicit-user")
		require.NoError(t, err, "Failed to create test user")

		req := map[string]any{
			"userId":   newUser.ID,
			"username": "explicituser",
			"uid":      2000,
		}

		resp, err := suite.Post("/systems/"+systemID+"/users", req)
		require.NoError(t, err, "Failed to create system user")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusCreated, resp.StatusCode,
			"Expected 201 Created, got %d", resp.StatusCode)

		var result map[string]any
		err = suite.ReadJSON(resp, &result)
		require.NoError(t, err, "Failed to read response JSON")

		sysUser, ok := result["user"].(map[string]any)
		require.True(t, ok, "Response should contain user object")

		uid, ok := sysUser["uid"].(float64)
		require.True(t, ok, "UID should be a number")
		assert.Equal(t, 2000, int(uid), "UID should be the specified value")
	})

	t.Run("rejects duplicate user in same system", func(t *testing.T) {
		// Create a real user in the database first
		newUser, err := suite.CreateTestUser(ctx, "test-provider", "dup-user-provider-id", "dup-user")
		require.NoError(t, err, "Failed to create test user")

		// First create should succeed
		req := map[string]any{
			"userId":   newUser.ID,
			"username": "dupuser",
		}
		resp1, err := suite.Post("/systems/"+systemID+"/users", req)
		require.NoError(t, err)
		_ = resp1.Body.Close()
		require.Equal(t, http.StatusCreated, resp1.StatusCode)

		// Second create with same userId should fail
		resp2, err := suite.Post("/systems/"+systemID+"/users", req)
		require.NoError(t, err)
		defer func() { _ = resp2.Body.Close() }()

		assert.Equal(t, http.StatusConflict, resp2.StatusCode,
			"Expected 409 Conflict for duplicate user, got %d", resp2.StatusCode)
	})

	t.Run("rejects duplicate username in same system", func(t *testing.T) {
		// Create two real users in the database
		user1, err := suite.CreateTestUser(ctx, "test-provider", "dupname1-provider-id", "dupname1-user")
		require.NoError(t, err, "Failed to create test user 1")

		user2, err := suite.CreateTestUser(ctx, "test-provider", "dupname2-provider-id", "dupname2-user")
		require.NoError(t, err, "Failed to create test user 2")

		req1 := map[string]any{
			"userId":   user1.ID,
			"username": "dupusername",
		}
		resp1, err := suite.Post("/systems/"+systemID+"/users", req1)
		require.NoError(t, err)
		_ = resp1.Body.Close()
		require.Equal(t, http.StatusCreated, resp1.StatusCode)

		// Same username, different userId
		req2 := map[string]any{
			"userId":   user2.ID,
			"username": "dupusername",
		}
		resp2, err := suite.Post("/systems/"+systemID+"/users", req2)
		require.NoError(t, err)
		defer func() { _ = resp2.Body.Close() }()

		assert.Equal(t, http.StatusConflict, resp2.StatusCode,
			"Expected 409 Conflict for duplicate username, got %d", resp2.StatusCode)
	})
}

// TestSystemUser_List tests listing system users
func TestSystemUser_List(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	err = suite.StartServer(ctx)
	require.NoError(t, err, "Failed to start server")

	// Setup user and system via API
	env, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	// Create additional system users (with real users in database)
	for i := 0; i < 3; i++ {
		newUser, err := suite.CreateTestUser(ctx, "test-provider", "list-user-provider-id-"+string(rune('a'+i)), "list-user-"+string(rune('a'+i)))
		require.NoError(t, err, "Failed to create test user")

		req := map[string]any{
			"userId":   newUser.ID,
			"username": "listuser" + string(rune('a'+i)),
		}
		resp, err := suite.Post("/systems/"+systemID+"/users", req)
		require.NoError(t, err)
		_ = resp.Body.Close()
	}

	t.Run("lists all users in system", func(t *testing.T) {
		resp, err := suite.Get("/systems/" + systemID + "/users")
		require.NoError(t, err, "Failed to list system users")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusOK, resp.StatusCode,
			"Expected 200 OK, got %d", resp.StatusCode)

		var result map[string]any
		err = suite.ReadJSON(resp, &result)
		require.NoError(t, err, "Failed to read response JSON")

		// Check users array
		users, ok := result["users"].([]any)
		if !ok {
			users, ok = result["data"].([]any)
		}
		require.True(t, ok, "Response should contain users array")
		assert.GreaterOrEqual(t, len(users), 3,
			"Should have at least 3 users")
	})
}

// TestSystemUser_Delete tests deleting a system user
func TestSystemUser_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	err = suite.StartServer(ctx)
	require.NoError(t, err, "Failed to start server")

	// Setup user and system via API
	env, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	// Create a real user in the database first
	newUser, err := suite.CreateTestUser(ctx, "test-provider", "delete-user-provider-id", "delete-user")
	require.NoError(t, err, "Failed to create test user")

	// Create a system user to delete
	req := map[string]any{
		"userId":   newUser.ID,
		"username": "deleteuser",
	}
	createResp, err := suite.Post("/systems/"+systemID+"/users", req)
	require.NoError(t, err)
	defer func() { _ = createResp.Body.Close() }()
	require.Equal(t, http.StatusCreated, createResp.StatusCode, "Expected 201 Created for user to delete")

	var createResult map[string]any
	err = suite.ReadJSON(createResp, &createResult)
	require.NoError(t, err, "Failed to read create response JSON")

	sysUser, ok := createResult["user"].(map[string]any)
	require.True(t, ok, "Response should contain user object, got: %v", createResult)
	uid := int(sysUser["uid"].(float64))

	t.Run("deletes user by UID", func(t *testing.T) {
		resp, err := suite.Delete("/systems/" + systemID + "/users/" + strconv.Itoa(uid))
		require.NoError(t, err, "Failed to delete system user")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode,
			"Expected 204 No Content, got %d", resp.StatusCode)

		// Verify user is deleted
		getResp, err := suite.Get("/systems/" + systemID + "/users/" + strconv.Itoa(uid))
		require.NoError(t, err)
		_ = getResp.Body.Close()
		assert.Equal(t, http.StatusNotFound, getResp.StatusCode,
			"User should not exist after deletion")
	})

	t.Run("returns 404 for non-existent UID", func(t *testing.T) {
		resp, err := suite.Delete("/systems/" + systemID + "/users/99999")
		require.NoError(t, err, "Failed to make delete request")
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"Expected 404 Not Found, got %d", resp.StatusCode)
	})
}

// TestSystemGroup tests group management
func TestSystemGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite := NewSuite(t)
	err := suite.Start(ctx)
	require.NoError(t, err, "Failed to start test suite")
	t.Cleanup(func() { _ = suite.Stop(ctx) })

	err = suite.StartServer(ctx)
	require.NoError(t, err, "Failed to start server")

	// Setup system via API (user is already created as root when system is initialized)
	env, err := suite.SetupFullEnvironmentAPI(ctx, "testuser")
	require.NoError(t, err, "Failed to setup full environment")
	systemID := env.SystemID

	t.Run("creates group with auto-assigned GID", func(t *testing.T) {
		req := map[string]any{
			"name": "developers",
		}

		resp, err := suite.Post("/systems/"+systemID+"/groups", req)
		require.NoError(t, err, "Failed to create group")
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusCreated, resp.StatusCode,
			"Expected 201 Created, got %d", resp.StatusCode)

		var result map[string]any
		err = suite.ReadJSON(resp, &result)
		require.NoError(t, err, "Failed to read response JSON")

		group, ok := result["group"].(map[string]any)
		if !ok {
			// Maybe not wrapped
			group = result
		}

		gid, ok := group["gid"].(float64)
		require.True(t, ok, "GID should be a number")
		assert.GreaterOrEqual(t, int(gid), 1000, "Auto-assigned GID should start from 1000")
	})

	t.Run("adds user to group", func(t *testing.T) {
		// First create a group
		grpReq := map[string]any{
			"name": "testgroup",
		}
		grpResp, err := suite.Post("/systems/"+systemID+"/groups", grpReq)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, grpResp.StatusCode, "Expected 201 Created for group")

		var grpResult map[string]any
		err = suite.ReadJSON(grpResp, &grpResult)
		_ = grpResp.Body.Close()
		require.NoError(t, err, "Failed to read group response JSON")

		group, ok := grpResult["group"].(map[string]any)
		require.True(t, ok, "Response should contain group object, got: %v", grpResult)
		gid := int(group["gid"].(float64))

		// Create a new user using the test helper (not API which requires OIDC fields)
		newUser, err := suite.CreateTestUser(ctx, "test-provider", "groupuser-test-provider-id", "groupuser-test-user")
		require.NoError(t, err, "Failed to create test user")
		newUserID := newUser.ID

		// Create a system user
		userReq := map[string]any{
			"userId":   newUserID,
			"username": "groupuser",
		}
		userResp, err := suite.Post("/systems/"+systemID+"/users", userReq)
		require.NoError(t, err)
		if userResp.StatusCode != http.StatusCreated {
			body := suite.ReadBody(userResp)
			t.Fatalf("Expected 201 Created for user, got %d: %s", userResp.StatusCode, body)
			return
		}

		var userResult map[string]any
		err = suite.ReadJSON(userResp, &userResult)
		_ = userResp.Body.Close()
		require.NoError(t, err, "Failed to read user response JSON")

		sysUser, ok := userResult["user"].(map[string]any)
		require.True(t, ok, "Response should contain user object, got: %v", userResult)
		uid := int(sysUser["uid"].(float64))

		// Add user to group
		addResp, err := suite.Post("/systems/"+systemID+"/groups/"+strconv.Itoa(gid)+"/members/"+strconv.Itoa(uid), nil)
		require.NoError(t, err, "Failed to add user to group")
		defer func() { _ = addResp.Body.Close() }()

		assert.Equal(t, http.StatusNoContent, addResp.StatusCode,
			"Expected 204 No Content, got %d", addResp.StatusCode)
	})
}
