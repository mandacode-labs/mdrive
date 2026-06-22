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
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type Server struct {
	app  *app.App
	http *http.Server
	addr string
}

func NewServer(a *app.App, fs handler.FSClient) *Server {
	cookieCfg := handler.CookieConfig{
		Name:     a.Cfg.HTTP.Cookie.Name,
		Path:     a.Cfg.HTTP.Cookie.Path,
		Secure:   a.Cfg.HTTP.Cookie.Secure,
		HttpOnly: a.Cfg.HTTP.Cookie.HttpOnly,
		SameSite: a.Cfg.HTTP.Cookie.SameSiteMode(),
	}
	healthDeps := handler.HealthDeps{
		DB: a.DB,
	}
	if a.SessionStore != nil {
		if s, ok := a.SessionStore.(interface {
			Scan(context.Context, func(string) error) error
		}); ok {
			healthDeps.ValKey = scannerAdapter{scan: s.Scan}
		}
	}
	if a.Perm != nil {
		healthDeps.Perm = a.Perm
	}
	h := handler.New(fs, func(ctx context.Context) (string, bool) {
		return "", false
	}, handler.WithDefaultStorage(drive.StorageConfig{
		Bucket:       a.Cfg.Storage.Bucket,
		Endpoint:     nil,
		Region:       a.Cfg.Storage.Region,
		AccessKey:    a.Cfg.Storage.AccessKey,
		SecretKey:    a.Cfg.Storage.SecretKey,
		UsePathStyle: a.Cfg.Storage.UsePathStyle,
	}), handler.WithCookie(cookieCfg), handler.WithHealthDeps(healthDeps))

	if a.Auth != nil && a.Security != nil {
		h.WithAuth(a.Auth, a.Cfg.Auth.FrontendURL)
	}

	var securityHandler api.SecurityHandler = &noopSecurity{}
	if a.Security != nil {
		securityHandler = a.Security
	}

	ogenServer, err := api.NewServer(h, securityHandler, api.WithErrorHandler(errorHandler))
	if err != nil {
		a.Log.Fatal().Err(err).Msg("failed to create ogen server")
	}

	var finalHandler = RequestIDMiddleware(ogenServer)
	if a.Security != nil {
		finalHandler = RequestIDMiddleware(a.Security.Middleware(ogenServer))
	}
	finalHandler = RequestIDMiddleware(withCORS(finalHandler, a.Cfg.HTTP.CORS))

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", a.Cfg.HTTP.Host, a.Cfg.HTTP.Port),
		Handler:           finalHandler,
		ReadHeaderTimeout: a.Cfg.HTTP.ReadTimeout,
		ReadTimeout:       a.Cfg.HTTP.ReadTimeout,
		WriteTimeout:      a.Cfg.HTTP.WriteTimeout,
		IdleTimeout:       a.Cfg.HTTP.IdleTimeout,
	}
	return &Server{
		app:  a,
		http: srv,
		addr: srv.Addr,
	}
}

func (s *Server) Run() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.addr, err)
	}
	s.app.Log.Info().Str("addr", s.addr).Msg("starting api server")
	go func() {
		if err := s.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.app.Log.Error().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	s.app.Log.Info().Msg("received shutdown signal")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.Shutdown(shutdownCtx)
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.http.Shutdown(ctx); err != nil {
		s.app.Log.Error().Err(err).Msg("http shutdown error")
	}
	if err := s.app.Close(); err != nil {
		s.app.Log.Error().Err(err).Msg("app close error")
	}
	s.app.Log.Info().Msg("api server stopped")
	return nil
}

func errorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	WriteError(w, err)
}

type noopSecurity struct{}

func (n *noopSecurity) HandleBearerAuth(ctx context.Context, _ api.OperationName, _ api.BearerAuth) (context.Context, error) {
	return ctx, nil
}

// scannerAdapter bridges a runtime interface (with a Scan method) to the
// static session.Scanner type expected by the health check.
type scannerAdapter struct {
	scan func(context.Context, func(string) error) error
}

func (s scannerAdapter) Scan(ctx context.Context, fn func(string) error) error {
	return s.scan(ctx, fn)
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
			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
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
