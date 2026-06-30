package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mandacode-labs/mdrive/internal/app"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/handler"
	"github.com/mandacode-labs/mdrive/internal/auth"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/permission"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type Server struct {
	app  *app.App
	http *http.Server
	addr string
}

func NewServer(a *app.App, fs handler.FSClient, driveSvc handler.DriveClient, uploadSvc handler.UploadClient, userSvc *user.Service, perm permission.Authorizer) (*Server, error) {
	cookieCfg := a.Config.HTTP.Cookie
	healthDeps := handler.HealthDeps{DB: a.DB}
	if perm != nil {
		healthDeps.Authorizer = perm
	}
	h := handler.New(fs, driveSvc, userSvc, uploadSvc, perm, a.Config.Auth.RedirectURI, a.Config.Auth.PostLoginURL,
		handler.WithDefaultStorage(drive.StorageConfig{
			Bucket:       a.Config.Storage.Bucket,
			Region:       a.Config.Storage.Region,
			AccessKey:    a.Config.Storage.AccessKey,
			SecretKey:    a.Config.Storage.SecretKey,
			UsePathStyle: a.Config.Storage.UsePathStyle,
		}),
		handler.WithPresignTTL(a.Config.Storage.PresignTTL),
		handler.WithCookie(cookieCfg),
		handler.WithHealthDeps(healthDeps),
	)

	var securityHandler api.SecurityHandler = &AnonSecurity{}
	if a.Security != nil {
		securityHandler = a.Security
	}

	ogenServer, err := api.NewServer(h, securityHandler, api.WithErrorHandler(func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
		a.Log.Error("handler error", "method", r.Method, "path", r.URL.Path, "error", err)
		WriteError(w, err)
	}))
	if err != nil {
		return nil, fmt.Errorf("apiserver: create ogen server: %w", err)
	}

	var secured http.Handler = ogenServer
	if a.Auth != nil {
		secured = a.Auth.Middleware(ogenServer)
	}
	finalHandler := AuthPassthrough(secured, a.Auth, a.Config.Auth.PostLoginURL, a.Config.Auth.AllowedOrigins)
	finalHandler = OpenAPIPassthrough(finalHandler)
	finalHandler = RequestIDMiddleware(finalHandler)
	finalHandler = withCORS(finalHandler, a.Config.HTTP.CORS)

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", a.Config.HTTP.Host, a.Config.HTTP.Port),
		Handler:           finalHandler,
		ReadHeaderTimeout: a.Config.HTTP.ReadTimeout,
		ReadTimeout:       a.Config.HTTP.ReadTimeout,
		WriteTimeout:      a.Config.HTTP.WriteTimeout,
		IdleTimeout:       a.Config.HTTP.IdleTimeout,
	}
	return &Server{
		app:  a,
		http: srv,
		addr: srv.Addr,
	}, nil
}

func (s *Server) Run() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.addr, err)
	}
	s.app.Log.Info("starting api server", "addr", s.addr)
	go func() {
		if err := s.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.app.Log.Error("server error", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGINT)
	<-quit
	s.app.Log.Info("received shutdown signal")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.Shutdown(shutdownCtx)
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.http.Shutdown(ctx); err != nil {
		s.app.Log.Error("http shutdown error", "err", err)
	}
	if err := s.app.Close(); err != nil {
		s.app.Log.Error("app close error", "err", err)
	}
	s.app.Log.Info("api server stopped")
	return nil
}

func withCORS(next http.Handler, cfg config.CORSConfig) http.Handler {
	if !cfg.Enabled {
		return next
	}

	allowedMethods := strings.Join(cfg.AllowedMethods, ", ")
	allowedHeaders := strings.Join(cfg.AllowedHeaders, ", ")
	exposedHeaders := strings.Join(cfg.ExposedHeaders, ", ")
	origins := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		origins[o] = struct{}{}
	}
	allowAll := false
	if _, ok := origins["*"]; ok {
		allowAll = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			switch {
			case allowAll && !cfg.AllowCredentials:
				w.Header().Set("Access-Control-Allow-Origin", "*")
			case allowAll && cfg.AllowCredentials:
				w.Header().Set("Access-Control-Allow-Origin", origin)
			default:
				if _, ok := origins[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}
			if cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", strconv.FormatBool(cfg.AllowCredentials))
			}
			if exposedHeaders != "" {
				w.Header().Set("Access-Control-Expose-Headers", exposedHeaders)
			}
		}

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
			if allowedHeaders != "" {
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			}
			if cfg.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AuthPassthrough routes OIDC login/callback/logout to the auth
// Service and forwards everything else to next. Mounted in
// the middleware chain BEFORE the ogen server so ogen never sees
// the auth-flow paths. When auth is not configured, the request
// is forwarded as-is.
//
// For /auth/login, extracts ?redirect_uri= and validates it against
// the AllowedOrigins allowlist. This is the post-login redirect
// target where the user lands after authentication. Without this
// filter, a malicious site could craft a link with an evil
// redirect_uri and phish the user.
func AuthPassthrough(next http.Handler, auth *auth.Service, postLoginURL string, allowedOrigins []string) http.Handler {
	if auth == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/login":
			auth.Authenticate(w, r, resolveRedirectURI(r, postLoginURL, allowedOrigins))
			return
		case "/auth/callback":
			auth.Callback(w, r)
			return
		case "/auth/logout":
			auth.Logout(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// resolveRedirectURI validates the caller-supplied ?redirect_uri
// against the allowed-origins allowlist. Empty, unparseable, or
// disallowed targets fall back to the safe post-login URL.
// Only https:// scheme with a matching host is accepted; relative
// paths (no host) are treated as safe; protocol-relative "//evil"
// URLs are rejected.
func resolveRedirectURI(r *http.Request, fallback string, allowed []string) string {
	target := r.URL.Query().Get("redirect_uri")
	if isAllowedRedirect(target, allowed) {
		return target
	}
	return fallback
}

func isAllowedRedirect(target string, allowed []string) bool {
	if target == "" {
		return false
	}
	if strings.HasPrefix(target, "//") {
		return false
	}
	u, err := url.Parse(target)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "" {
		return u.Scheme == ""
	}
	if u.Scheme != "https" {
		return false
	}
	for _, a := range allowed {
		if strings.EqualFold(host, a) {
			return true
		}
	}
	return false
}
