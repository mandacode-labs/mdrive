package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mandacode-labs/mdrive/internal/app"
	"github.com/mandacode-labs/mdrive/internal/app/apiserver/handler"
	"github.com/mandacode-labs/mdrive/pkg/api"
)

type Server struct {
	app  *app.App
	http *http.Server
	addr string
}

func NewServer(a *app.App, fs handler.FS) *Server {
	h := handler.New(fs, func(ctx context.Context) (string, bool) {
		// Fallback: no user extraction by default (auth handles it via session context)
		return "", false
	})

	if a.Auth != nil && a.Security != nil {
		h.WithAuth(a.Auth, a.Cfg.Auth.FrontendURL, a.Cfg.App.Env != "development")
	}

	var securityHandler api.SecurityHandler = &noopSecurity{}
	if a.Security != nil {
		securityHandler = a.Security
	}

	ogenServer, err := api.NewServer(h, securityHandler, api.WithErrorHandler(errorHandler))
	if err != nil {
		a.Log.Fatal().Err(err).Msg("failed to create ogen server")
	}

	var finalHandler http.Handler = ogenServer
	if a.Security != nil {
		finalHandler = a.Security.Middleware(ogenServer)
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", a.Cfg.HTTP.Host, a.Cfg.HTTP.Port),
		Handler:           finalHandler,
		ReadHeaderTimeout: 30 * time.Second,
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
