package proxy

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/adamdrake/go_proxy/internal/capture"
)

// Config holds the proxy server configuration
type Config struct {
	ListenAddr     string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxRequestSize int64
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		ListenAddr:     ":8080",
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxRequestSize: 10 * 1024 * 1024, // 10MB
	}
}

// Server is the main proxy server
type Server struct {
	config  Config
	store   *capture.Store
	handler *Handler
	server  *http.Server
}

// NewServer creates a new proxy server
func NewServer(config Config, store *capture.Store) *Server {
	handler := NewHandler(store, config.MaxRequestSize)

	return &Server{
		config:  config,
		store:   store,
		handler: handler,
	}
}

// Start begins listening for proxy requests
func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:         s.config.ListenAddr,
		Handler:      s.handler,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		// Disable HTTP/2 for proxy compatibility
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	listener, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return err
	}

	log.Printf("Proxy server listening on %s", s.config.ListenAddr)

	return s.server.Serve(listener)
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// Store returns the capture store
func (s *Server) Store() *capture.Store {
	return s.store
}
