package apiserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/cors"

	"github.com/mandacode-labs/mdrive/internal/app"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/handler"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/middleware"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/spec"
	"github.com/mandacode-labs/mdrive/internal/config"
	"github.com/mandacode-labs/mdrive/internal/core/drive"
	"github.com/mandacode-labs/mdrive/internal/core/user"
	"github.com/mandacode-labs/mdrive/internal/errorx"
	"github.com/mandacode-labs/mdrive/internal/logx"
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
	h := handler.New(fs, driveSvc, userSvc, uploadSvc, perm, a.Config.Auth.RedirectURI,
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

	ogenServer, err := api.NewServer(h, securityHandler,
		api.WithMiddleware(middleware.ErrorMiddleware(), middleware.PanicMiddleware()),
	)
	if err != nil {
		return nil, errorx.Wrap(err, "apiserver: create ogen server")
	}

	finalHandler := buildChain(a, ogenServer)

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

// buildChain assembles path-specific mounts and cross-cutting
// middleware. From outermost to innermost: CSRF, CORS, http.ServeMux
// (OIDC flow, /openapi.json, ogen).
func buildChain(a *app.App, ogenServer http.Handler) http.Handler {
	mux := http.NewServeMux()
	if a.Auth != nil {
		mux.Handle("/auth/login", authHandler(a.Auth.Authenticate, "login"))
		mux.Handle("/auth/callback", authHandler(a.Auth.Callback, "callback"))
		mux.Handle("/auth/logout", authHandler(a.Auth.Logout, "logout"))
	}
	mux.Handle("/openapi.json", spec.Handler())
	mux.Handle("/", ogenServer)

	csrf := middleware.CSRFMiddleware(middleware.CSRFConfig{
		AllowedOrigins: a.Config.HTTP.CORS.AllowedOrigins,
	})(mux)

	return newCORS(a.Config.HTTP.CORS).Handler(csrf)
}

func newCORS(cfg config.CORSConfig) *cors.Cors {
	return cors.New(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   cfg.AllowedMethods,
		AllowedHeaders:   cfg.AllowedHeaders,
		ExposedHeaders:   cfg.ExposedHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           cfg.MaxAge,
	})
}

func authHandler(fn func(http.ResponseWriter, *http.Request), flow string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logx.Debug(r.Context(), "apiserver.auth."+flow)
		fn(w, r)
	})
}

func (s *Server) Run() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return errorx.Wrap(err, fmt.Sprintf("apiserver: listen (addr=%s)", s.addr))
	}
	logx.Info(context.Background(), "api_server.starting",
		slog.String("addr", s.addr),
	)
	go func() {
		if err := s.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logx.Error(context.Background(), err, "api_server.serve_error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGINT)
	<-quit
	logx.Info(context.Background(), "api_server.shutdown_signal")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.Shutdown(shutdownCtx)
}

func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.http.Shutdown(ctx); err != nil {
		logx.Error(ctx, err, "apiserver.http_shutdown_error")
	}
	if err := s.app.Close(); err != nil {
		logx.Error(ctx, err, "apiserver.app_close_error")
	}
	logx.Info(ctx, "api_server.stopped")
	return nil
}
