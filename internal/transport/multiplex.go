// Package transport provides yamux multiplexing support
package transport

import (
	"net"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
)

// MultiplexConfig holds multiplexer configuration
type MultiplexConfig struct {
	AcceptBacklog          int
	EnableKeepAlive        bool
	KeepAliveInterval      time.Duration
	ConnectionWriteTimeout time.Duration
	MaxStreamWindowSize    uint32
}

// DefaultMultiplexConfig returns the default multiplexer configuration
func DefaultMultiplexConfig() *MultiplexConfig {
	return &MultiplexConfig{
		AcceptBacklog:          256,
		EnableKeepAlive:        true,
		KeepAliveInterval:      30 * time.Second,
		ConnectionWriteTimeout: 10 * time.Second,
		MaxStreamWindowSize:    256 * 1024,
	}
}

// Multiplexer wraps yamux session for connection multiplexing
type Multiplexer struct {
	session *yamux.Session
	config  *yamux.Config
	mu      sync.Mutex
	closed  bool
}

// NewMultiplexer creates a new multiplexer over the given connection
func NewMultiplexer(conn net.Conn, isServer bool, cfg *MultiplexConfig) (*Multiplexer, error) {
	if cfg == nil {
		cfg = DefaultMultiplexConfig()
	}

	yamuxConfig := yamux.DefaultConfig()
	yamuxConfig.AcceptBacklog = cfg.AcceptBacklog
	yamuxConfig.EnableKeepAlive = cfg.EnableKeepAlive
	yamuxConfig.KeepAliveInterval = cfg.KeepAliveInterval
	yamuxConfig.ConnectionWriteTimeout = cfg.ConnectionWriteTimeout
	yamuxConfig.MaxStreamWindowSize = cfg.MaxStreamWindowSize

	var session *yamux.Session
	var err error

	if isServer {
		session, err = yamux.Server(conn, yamuxConfig)
	} else {
		session, err = yamux.Client(conn, yamuxConfig)
	}

	if err != nil {
		return nil, err
	}

	return &Multiplexer{
		session: session,
		config:  yamuxConfig,
	}, nil
}

// Open opens a new stream
func (m *Multiplexer) Open() (Conn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, yamux.ErrSessionShutdown
	}

	conn, err := m.session.Open()
	if err != nil {
		return nil, err
	}

	return &streamConn{conn: conn}, nil
}

// Accept waits for and returns the next stream
func (m *Multiplexer) Accept() (Conn, error) {
	conn, err := m.session.Accept()
	if err != nil {
		return nil, err
	}

	return &streamConn{conn: conn}, nil
}

// Close closes the multiplexer
func (m *Multiplexer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	return m.session.Close()
}

// IsClosed returns whether the multiplexer is closed
func (m *Multiplexer) IsClosed() bool {
	return m.session.IsClosed()
}

// NumStreams returns the number of open streams
func (m *Multiplexer) NumStreams() int {
	return m.session.NumStreams()
}

// streamConn wraps net.Conn to implement Conn
type streamConn struct {
	conn net.Conn
}

func (s *streamConn) Read(p []byte) (int, error) {
	return s.conn.Read(p)
}

func (s *streamConn) Write(p []byte) (int, error) {
	return s.conn.Write(p)
}

func (s *streamConn) Close() error {
	return s.conn.Close()
}

func (s *streamConn) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *streamConn) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *streamConn) SetDeadline(t time.Time) error {
	return s.conn.SetDeadline(t)
}

func (s *streamConn) SetReadDeadline(t time.Time) error {
	return s.conn.SetReadDeadline(t)
}

func (s *streamConn) SetWriteDeadline(t time.Time) error {
	return s.conn.SetWriteDeadline(t)
}
