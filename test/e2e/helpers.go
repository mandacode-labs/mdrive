package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/ent"
	"github.com/mandacode-labs/mdrive/ent/object"
	"github.com/mandacode-labs/mdrive/ent/system"
	domainsession "github.com/mandacode-labs/mdrive/internal/session"
	"github.com/mandacode-labs/mdrive/internal/session/repository"
)

// HTTP Methods

func (s *Suite) Get(path string) (*http.Response, error) {
	return s.Do("GET", path, nil)
}

func (s *Suite) Post(path string, body any) (*http.Response, error) {
	return s.Do("POST", path, body)
}

func (s *Suite) Put(path string, body any) (*http.Response, error) {
	return s.Do("PUT", path, body)
}

func (s *Suite) Patch(path string, body any) (*http.Response, error) {
	return s.Do("PATCH", path, body)
}

func (s *Suite) Delete(path string) (*http.Response, error) {
	return s.Do("DELETE", path, nil)
}

func (s *Suite) Do(method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = strings.NewReader(string(jsonData))
	}

	fullURL := s.BaseURL() + path
	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for _, cookie := range s.cookieJar {
		req.AddCookie(cookie)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	s.cookieJar = append(s.cookieJar, resp.Cookies()...)

	return resp, nil
}

// Response Helpers

func (s *Suite) ReadJSON(resp *http.Response, v any) error {
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w (body: %s)", err, string(data))
	}
	return nil
}

func (s *Suite) ReadBody(resp *http.Response) string {
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	return string(data)
}

// Cookie Helpers

func (s *Suite) ClearCookies() {
	s.cookieJar = make([]*http.Cookie, 0)
}

func (s *Suite) AddCookie(cookie *http.Cookie) {
	s.cookieJar = append(s.cookieJar, cookie)
}

// Test Data Helpers

func (s *Suite) CreateTestUser(ctx context.Context, provider, providerID, username string) (*ent.User, error) {
	u, err := s.EntClient.User.Create().
		SetID(uuid.New().String()).
		SetProvider(provider).
		SetProviderID(providerID).
		SetUsername(username).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create test user: %w", err)
	}
	return u, nil
}

func (s *Suite) CreateTestSystem(ctx context.Context, name string) (*ent.System, error) {
	sys, err := s.EntClient.System.Create().
		SetID(uuid.New().String()).
		SetName(name).
		SetStatus(system.StatusActive).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create test system: %w", err)
	}
	return sys, nil
}

func (s *Suite) CreateTestSession(ctx context.Context, userID string) (string, error) {
	sessionID := uuid.New().String()
	now := time.Now()
	expiresAt := now.Add(1 * time.Hour)

	sessionRepo := repository.NewValkeySessionRepository(s.ValkeyClient, "mdrive:session:")
	session := domainsession.NewSession(
		domainsession.SessionID(sessionID),
		userID,
		expiresAt,
		now,
		"", // no id_token in test sessions
	)

	if err := sessionRepo.Save(ctx, session); err != nil {
		return "", fmt.Errorf("failed to create test session: %w", err)
	}

	return sessionID, nil
}

func (s *Suite) CreateTestSystemUser(ctx context.Context, systemID, userID, username string, uid, gid int) (*ent.UserSystem, error) {
	su, err := s.EntClient.UserSystem.Create().
		SetSystemID(systemID).
		SetUserID(userID).
		SetUsername(username).
		SetUID(uid).
		SetGid(gid).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create test system user: %w", err)
	}
	return su, nil
}

func (s *Suite) CreateTestSystemGroup(ctx context.Context, systemID, name string, gid int) (*ent.SystemGroup, error) {
	sg, err := s.EntClient.SystemGroup.Create().
		SetSystemID(systemID).
		SetName(name).
		SetGid(gid).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create test system group: %w", err)
	}
	return sg, nil
}

func (s *Suite) CreateTestInode(ctx context.Context, systemID string, mode, uid, gid int, content []byte) (*ent.Inode, error) {
	in, err := s.EntClient.Inode.Create().
		SetSystemID(systemID).
		SetMode(mode).
		SetUID(uid).
		SetGid(gid).
		SetSize(int64(len(content))).
		SetLinkCount(1).
		SetContent(content).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create test inode: %w", err)
	}
	return in, nil
}

func (s *Suite) CreateTestObject(ctx context.Context, systemID, bucket, storageKey string) (*ent.Object, error) {
	obj, err := s.EntClient.Object.Create().
		SetID(uuid.New().String()).
		SetSystemID(systemID).
		SetProvider(object.ProviderS3).
		SetBucket(bucket).
		SetStorageKey(storageKey).
		SetStatus(object.StatusActive).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create test object: %w", err)
	}
	return obj, nil
}

// Authentication Helpers

func (s *Suite) LoginAs(ctx context.Context, userID string) error {
	sessionID, err := s.CreateTestSession(ctx, userID)
	if err != nil {
		return err
	}
	// #nosec G124 — test environment cookie, not production
	sessionCookie := &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // test environment
		SameSite: http.SameSiteLaxMode,
	}
	s.cookieJar = append(s.cookieJar, sessionCookie)
	return nil
}

func (s *Suite) Logout() {
	s.ClearCookies()
}

// Environment Setup Helpers

func (s *Suite) SetupAuthenticatedUser(ctx context.Context, username string) (*ent.User, error) {
	provider := "keycloak"
	providerID := fmt.Sprintf("test-%s", username)

	u, err := s.CreateTestUser(ctx, provider, providerID, username)
	if err != nil {
		return nil, err
	}

	if err := s.LoginAs(ctx, u.ID); err != nil {
		return nil, err
	}

	return u, nil
}

func (s *Suite) SetupSystemWithUser(ctx context.Context, systemName string, userID, username string) (*ent.System, *ent.UserSystem, error) {
	sys, err := s.CreateTestSystem(ctx, systemName)
	if err != nil {
		return nil, nil, err
	}

	_, err = s.CreateTestSystemGroup(ctx, sys.ID, "root", 0)
	if err != nil {
		return nil, nil, err
	}

	_, err = s.CreateTestSystemGroup(ctx, sys.ID, username, 1000)
	if err != nil {
		return nil, nil, err
	}

	su, err := s.CreateTestSystemUser(ctx, sys.ID, userID, username, 1000, 1000)
	if err != nil {
		return nil, nil, err
	}

	return sys, su, nil
}

func (s *Suite) SetupFullEnvironment(ctx context.Context, username string) (*ent.User, *ent.System, *ent.UserSystem, error) {
	u, err := s.SetupAuthenticatedUser(ctx, username)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to setup authenticated user: %w", err)
	}

	sys, su, err := s.SetupSystemWithUser(ctx, fmt.Sprintf("%s-system", username), u.ID, username)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to setup system: %w", err)
	}

	return u, sys, su, nil
}

// TestEnvironment holds the result of setting up a full test environment.
type TestEnvironment struct {
	User     *ent.User
	SystemID string
}

// SetupFullEnvironmentAPI creates a system via the API (which initializes filesystem)
// and returns the created environment. This is the preferred method for e2e tests.
func (s *Suite) SetupFullEnvironmentAPI(ctx context.Context, username string) (*TestEnvironment, error) {
	// Setup authenticated user
	u, err := s.SetupAuthenticatedUser(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to setup authenticated user: %w", err)
	}

	// Create system via API (this initializes filesystem with root directory, /home, etc.)
	req := map[string]any{
		"name":        fmt.Sprintf("%s-system", username),
		"description": "Test system for e2e tests",
	}

	resp, err := s.Post("/systems", req)
	if err != nil {
		return nil, fmt.Errorf("failed to create system: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body := s.ReadBody(resp)
		return nil, fmt.Errorf("failed to create system: status=%d body=%s", resp.StatusCode, body)
	}

	var result map[string]any
	if err := s.ReadJSON(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse system response: %w", err)
	}

	systemData, ok := result["system"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid system response format")
	}

	systemID, ok := systemData["id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid system id format")
	}

	return &TestEnvironment{
		User:     u,
		SystemID: systemID,
	}, nil
}

// Upload Helpers

// InitiateUpload initiates an upload, uploads test content to the presigned URL,
// and returns the object ID.
func (s *Suite) InitiateUpload(t *testing.T, systemID, path string) string {
	t.Helper()
	data := []byte("test content")

	req := map[string]any{
		"path":        path,
		"contentType": "text/plain",
		"size":        int64(len(data)),
	}
	resp, err := s.Post("/fs/"+systemID+"/upload/initiate", req)
	require.NoError(t, err, "Failed to initiate upload")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "Initiate should succeed")

	var result map[string]any
	require.NoError(t, s.ReadJSON(resp, &result), "Failed to read response JSON")

	session, ok := result["uploadSession"].(map[string]any)
	require.True(t, ok, "Response should contain uploadSession")

	uploadURL := session["uploadUrl"].(string)
	s.UploadToPresignedURL(t, uploadURL, data)

	return session["objectId"].(string)
}

// InitiateUploadWithURL initiates an upload, uploads test content to the presigned URL,
// and returns both the object ID and upload URL.
func (s *Suite) InitiateUploadWithURL(t *testing.T, systemID, path string) (objectID, uploadURL string) {
	t.Helper()
	data := []byte("test content")

	req := map[string]any{
		"path":        path,
		"contentType": "text/plain",
		"size":        int64(len(data)),
	}
	resp, err := s.Post("/fs/"+systemID+"/upload/initiate", req)
	require.NoError(t, err, "Failed to initiate upload")
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusCreated, resp.StatusCode, "Initiate should succeed")

	var result map[string]any
	require.NoError(t, s.ReadJSON(resp, &result), "Failed to read response JSON")

	session, ok := result["uploadSession"].(map[string]any)
	require.True(t, ok, "Response should contain uploadSession")

	objectID = session["objectId"].(string)
	uploadURL = session["uploadUrl"].(string)

	// Upload actual content to MinIO via presigned URL
	s.UploadToPresignedURL(t, uploadURL, data)

	return objectID, uploadURL
}

// URL Helpers

func (s *Suite) BuildURL(path string, params ...any) string {
	return s.BaseURL() + fmt.Sprintf(path, params...)
}

func (s *Suite) BuildURLWithQuery(path string, query url.Values) string {
	if len(query) == 0 {
		return s.BaseURL() + path
	}
	return s.BaseURL() + path + "?" + query.Encode()
}

// MinIO Helpers

// UploadToPresignedURL uploads data to a presigned URL (for testing MinIO uploads).
// Go's http.NewRequest automatically sets Content-Length when using bytes.NewReader.
func (s *Suite) UploadToPresignedURL(t *testing.T, presignedURL string, data []byte) {
	t.Logf("Uploading to presigned URL: %s", presignedURL)

	req, err := http.NewRequest("PUT", presignedURL, bytes.NewReader(data))
	require.NoError(t, err, "Failed to create upload request")
	req.Header.Set("Content-Type", "text/plain")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err, "Failed to upload to presigned URL")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Upload to presigned URL failed with status %d: %s\nURL: %s", resp.StatusCode, string(body), presignedURL)
	}
}
