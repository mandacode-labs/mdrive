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

	"github.com/mandacode-labs/mdrive/internal/app"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/handler"
	"github.com/mandacode-labs/mdrive/internal/auth"
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

	ogenServer, err := api.NewServer(h, securityHandler, api.WithErrorHandler(func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
		logx.Error(ctx, err, "handler error",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
		)
		WriteError(w, err)
	}))
	if err != nil {
		return nil, errorx.Wrap(err, "apiserver: create ogen server")
	}

	var secured http.Handler = ogenServer
	if a.Auth != nil {
		secured = a.Auth.AuthBridge(ogenServer)
	}
	finalHandler := AuthPassthrough(secured, a.Auth)
	finalHandler = OpenAPIPassthrough(finalHandler)
	finalHandler = RequestIDMiddleware(finalHandler)
	finalHandler = withCORS(finalHandler, a.Config.HTTP.CORS)
	finalHandler = withRequestLog(finalHandler)
	finalHandler = recoverPanic(finalHandler)

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
		logx.Error(ctx, err, "api_server.http_shutdown_error")
	}
	if err := s.app.Close(); err != nil {
		logx.Error(ctx, err, "api_server.app_close_error")
	}
	logx.Info(ctx, "api_server.stopped")
	return nil
}

// AuthPassthrough routes OIDC login/callback/logout to the auth
// Service and forwards everything else to next. Mounted in the
// middleware chain BEFORE the ogen server so ogen never sees the
// auth-flow paths. When auth is not configured, the request is
// forwarded as-is.
func AuthPassthrough(next http.Handler, auth *auth.Service) http.Handler {
	if auth == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/login":
			logx.Debug(r.Context(), "apiserver.auth.login")
			auth.Authenticate(w, r)
			return
		case "/auth/callback":
			logx.Debug(r.Context(), "apiserver.auth.callback")
			auth.Callback(w, r)
			return
		case "/auth/logout":
			logx.Debug(r.Context(), "apiserver.auth.logout")
			auth.Logout(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
