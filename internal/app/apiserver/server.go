package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mandacode-labs/mdrive/internal/app"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/handler"
	"github.com/mandacode-labs/mdrive/internal/auth/session"
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
	healthDeps := handler.HealthDeps{
		DB: a.DB,
	}
	if s, ok := a.SessionStore.(session.Scanner); ok {
		healthDeps.Valkey = s
	}
	if perm != nil {
		healthDeps.Authorizer = perm
	}
	h := handler.New(fs, driveSvc, userSvc, uploadSvc, perm, a.Auth, a.Config.Auth.RedirectURI,
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

	ogenServer, err := api.NewServer(h, securityHandler, api.WithErrorHandler(errorHandler))
	if err != nil {
		return nil, fmt.Errorf("apiserver: create ogen server: %w", err)
	}

	// Build the middleware chain once. With SessionSecurity the
	// session-auth middleware wraps the ogen server; with
	// AnonSecurity it is a no-op. The openapi passthrough mounts
	// BEFORE the auth wrapper so /openapi.json is reachable without
	// a session; every other path is forwarded to the secured
	// ogen server. The requestID and CORS middlewares are applied
	// in the same chain.
	var secured http.Handler = ogenServer
	if a.Security != nil {
		secured = a.Security.Middleware(ogenServer)
	}
	finalHandler := OpenAPIPassthrough(secured)
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
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
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

func errorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	WriteError(w, err)
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
			if allowAll && !cfg.AllowCredentials {
				// Safe to set literal * when credentials are off.
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if allowAll && cfg.AllowCredentials {
				// Spec: Access-Control-Allow-Origin: * is forbidden
				// when credentials are true. Echo the request origin
				// for wildcard-like behaviour.
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else if _, ok := origins[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
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
