// Package apiserver provides the HTTP API server for mdrive.
//
// Responsibilities:
//   - Server lifecycle (start, graceful shutdown)
//   - Route registration
//   - Domain → HTTP error conversion
//   - Handler delegation
package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mandacode-labs/mdrive/internal/app"
)

// Server is the HTTP API server.
type Server struct {
	app    *app.App
	http   *http.Server
	addr   string
}

// NewServer creates a new Server with the given app.
func NewServer(a *app.App) *Server {
	router := newRouter(a)
	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", a.Cfg.HTTP.Host, a.Cfg.HTTP.Port),
		Handler:           router,
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

// newRouter constructs the HTTP router with all routes.
func newRouter(a *app.App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /version", versionHandler(a.Cfg.App.Env))
	return mux
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func versionHandler(env string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"env": env})
	}
}
