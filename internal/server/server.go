// Package server provides the server-side tunnel implementation
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sahmadiut/cloudtunnel/internal/config"
	"github.com/sahmadiut/cloudtunnel/internal/logger"
)

// Server represents a tunnel server
type Server struct {
	cfg     *config.Config
	log     *logger.Logger
	handler *Handler
	server  *http.Server

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new tunnel server
func New(cfg *config.Config, log *logger.Logger) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	handler := NewHandler(cfg, log)

	mux := http.NewServeMux()
	mux.Handle(cfg.Listen.Path, handler)

	server := &http.Server{
		Addr:         cfg.Listen.Address,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if cfg.TLS.Enabled {
		cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to load TLS certificates: %w", err)
		}

		server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	}

	return &Server{
		cfg:     cfg,
		log:     log,
		handler: handler,
		server:  server,
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// Start starts the tunnel server
func (s *Server) Start() error {
	s.log.Info().
		Str("address", s.cfg.Listen.Address).
		Str("path", s.cfg.Listen.Path).
		Bool("tls", s.cfg.TLS.Enabled).
		Msg("starting tunnel server")

	var err error

	if s.cfg.TLS.Enabled {
		s.log.Info().
			Str("cert", s.cfg.TLS.CertFile).
			Str("key", s.cfg.TLS.KeyFile).
			Msg("TLS enabled")
		err = s.server.ListenAndServeTLS("", "")
	} else {
		err = s.server.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Stop stops the tunnel server
func (s *Server) Stop() error {
	s.log.Info().Msg("stopping tunnel server")

	s.cancel()

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		s.log.Error().
			Err(err).
			Msg("error during server shutdown")
		return err
	}

	s.log.Info().Msg("tunnel server stopped")
	return nil
}

// Wait blocks until the server is stopped
func (s *Server) Wait() {
	<-s.ctx.Done()
}
