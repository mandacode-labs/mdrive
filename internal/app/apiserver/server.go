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

// Server is the HTTP API server.
type Server struct {
	app  *app.App
	http *http.Server
	addr string
}

// NewServer creates a new Server. It wires the VFS service as the ogen Handler
// and sets up the error handler for domain-to-HTTP conversion.
func NewServer(a *app.App, fs handler.FS, getUser func(context.Context) (string, bool)) *Server {
	h := handler.New(fs, getUser)

	ogenServer, err := api.NewServer(h, &noopSecurity{}, api.WithErrorHandler(errorHandler))
	if err != nil {
		a.Log.Fatal().Err(err).Msg("failed to create ogen server")
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", a.Cfg.HTTP.Host, a.Cfg.HTTP.Port),
		Handler:           ogenServer,
		ReadHeaderTimeout: 30 * time.Second,
	}
	return &Server{
		app:  a,
		http: srv,
		addr: srv.Addr,
	}
}

// Run starts the server and blocks until shutdown.
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

// Shutdown stops the HTTP server gracefully.
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

// errorHandler is the ogen-compatible error handler that converts domain
// errors to HTTP status codes and JSON error responses.
func errorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	WriteError(w, err)
}

// noopSecurity is a stub security handler.
type noopSecurity struct{}

func (n *noopSecurity) HandleBearerAuth(ctx context.Context, _ api.OperationName, _ api.BearerAuth) (context.Context, error) {
	return ctx, nil
}
