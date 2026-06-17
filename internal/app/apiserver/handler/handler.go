package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/auth/session"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/node"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/vfs"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type FSClient interface {
	Mkdir(ctx context.Context, userID, driveID, path string) (*node.Node, error)
	Touch(ctx context.Context, userID, driveID, path string) (*node.Node, error)
	Rm(ctx context.Context, userID, driveID string, paths []string, recursive bool) error
	Mv(ctx context.Context, userID, srcDriveID string, srcPaths []string, dstDriveID, dstPath string) error
	Ls(ctx context.Context, userID, driveID, path string) (node.DirContent, error)
	Cat(ctx context.Context, userID, driveID, path string) ([]byte, error)
	Write(ctx context.Context, userID, driveID, path, content string) error
	WriteLarge(ctx context.Context, userID, driveID, path string, obj node.ObjectContent, size int64) error
	Stat(ctx context.Context, userID, driveID, path string) (*node.Node, error)
	Symlink(ctx context.Context, userID, driveID, target, linkPath string) (*node.Node, error)
	InitiateUpload(ctx context.Context, userID, driveID, destPath string, contentType *string, contentLength *int64, expiry time.Duration) (vfs.PresignInfo, error)
	CompleteUpload(ctx context.Context, userID, driveID, uploadID string, contentLength int64, checksum *string) (*node.Node, error)
	PresignDownload(ctx context.Context, userID, driveID, path string, expiry time.Duration) (vfs.PresignInfo, error)
	CreateDrive(ctx context.Context, actorID string, name, description string, cfg drive.StorageConfig) (*drive.Drive, uuid.UUID, error)
	GetDrive(ctx context.Context, id string) (*drive.Drive, error)
	GetDriveStorage(ctx context.Context, driveID string) (*drive.Storage, error)
	UpdateDrive(ctx context.Context, id string, name, description *string) (*drive.Drive, error)
	DeleteDrive(ctx context.Context, id string) error
	ListUserDrives(ctx context.Context, actorID string) ([]*drive.Drive, error)
	UpsertUser(ctx context.Context, cmd *user.CreateCommand) (*user.User, error)
	GetUser(ctx context.Context, id string) (*user.User, error)
}

// AuthClient is the consumer-declared interface for authentication operations.
type AuthClient interface {
	ExchangeJWT(ctx context.Context, assertion string) (*oidc.Tokens[*oidc.IDTokenClaims], error)
	ExchangeCode(ctx context.Context, code, redirectURI, codeVerifier string) (*oidc.Tokens[*oidc.IDTokenClaims], error)
	AuthorizeURL(ctx context.Context, provider, redirectURI, state, codeChallenge string) (string, error)
	VerifyIDToken(ctx context.Context, raw string) (*oidc.IDTokenClaims, error)
	CreateSession(ctx context.Context, userID, provider string) (*session.Session, error)
	DeleteSession(ctx context.Context, id string) error
	StorePKCE(ctx context.Context, state, verifier string) error
	GetPKCE(ctx context.Context, state string) (string, error)
}

type Handler struct {
	vfs          FSClient
	getUser      func(context.Context) (string, bool)
	auth         AuthClient
	frontendURL  string
	secureCookie bool
}

func New(fs FSClient, getUser func(context.Context) (string, bool)) *Handler {
	return &Handler{vfs: fs, getUser: getUser}
}

func (h *Handler) WithAuth(a AuthClient, frontendURL string, secureCookie bool) {
	h.auth = a
	h.frontendURL = frontendURL
	h.secureCookie = secureCookie
}

func (h *Handler) userID(ctx context.Context) string {
	if id, ok := h.getUser(ctx); ok {
		return id
	}
	if h.auth != nil {
		sess := auth.SessionFromContext(ctx)
		if sess != nil {
			return sess.UserID
		}
	}
	return ""
}

// Compile-time checks.
var _ api.Handler = (*Handler)(nil)
var _ AuthClient = (*auth.Service)(nil)
var _ = fmt.Sprintf
