// Package transport provides WebSocket-based tunnel transport
package transport

import (
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sahmadiut/cloudtunnel/internal/config"
)

// WebSocketTransport implements Transport using WebSocket
type WebSocketTransport struct {
	cfg      *config.Config
	dialer   *websocket.Dialer
	serverWS string
	mu       sync.Mutex
	closed   bool
}

// NewWebSocketTransport creates a new WebSocket transport
func NewWebSocketTransport(cfg *config.Config) (*WebSocketTransport, error) {
	scheme := "ws"
	if cfg.TLS.Enabled {
		scheme = "wss"
	}

	serverWS := scheme + "://" + cfg.Server.Address + "/tunnel"

	dialer := &websocket.Dialer{
		HandshakeTimeout: cfg.Resilience.ConnectionTimeout,
		ReadBufferSize:   cfg.Transport.BufferSize,
		WriteBufferSize:  cfg.Transport.BufferSize,
	}

	return &WebSocketTransport{
		cfg:      cfg,
		dialer:   dialer,
		serverWS: serverWS,
	}, nil
}

// Dial establishes a new WebSocket connection
func (t *WebSocketTransport) Dial() (Conn, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, errors.New("transport is closed")
	}
	t.mu.Unlock()

	headers := http.Header{}
	if t.cfg.Security.AuthToken != "" {
		headers.Set("Authorization", "Bearer "+t.cfg.Security.AuthToken)
	}

	conn, _, err := t.dialer.Dial(t.serverWS, headers)
	if err != nil {
		return nil, err
	}

	return NewWebSocketConn(conn), nil
}

// Close closes the transport
func (t *WebSocketTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closed = true
	return nil
}

// WebSocketConn implements Conn over WebSocket
type WebSocketConn struct {
	conn       *websocket.Conn
	readBuf    []byte
	readPos    int
	mu         sync.Mutex
	closed     bool
	localAddr  net.Addr
	remoteAddr net.Addr
}

// NewWebSocketConn creates a new WebSocket connection wrapper
func NewWebSocketConn(conn *websocket.Conn) *WebSocketConn {
	return &WebSocketConn{
		conn:       conn,
		localAddr:  conn.LocalAddr(),
		remoteAddr: conn.RemoteAddr(),
	}
}

// Read reads data from the WebSocket connection
func (c *WebSocketConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If we have buffered data, return it
	if c.readPos < len(c.readBuf) {
		n := copy(p, c.readBuf[c.readPos:])
		c.readPos += n
		return n, nil
	}

	// Read new message from WebSocket
	_, data, err := c.conn.ReadMessage()
	if err != nil {
		return 0, err
	}

	c.readBuf = data
	c.readPos = 0

	n := copy(p, c.readBuf)
	c.readPos = n
	return n, nil
}

// Write writes data to the WebSocket connection
func (c *WebSocketConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

// Close closes the WebSocket connection
func (c *WebSocketConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.conn.Close()
}

// LocalAddr returns the local network address
func (c *WebSocketConn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr returns the remote network address
func (c *WebSocketConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// SetDeadline sets the read and write deadlines
func (c *WebSocketConn) SetDeadline(t time.Time) error {
	if err := c.conn.SetReadDeadline(t); err != nil {
		return err
	}
	return c.conn.SetWriteDeadline(t)
}

// SetReadDeadline sets the read deadline
func (c *WebSocketConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline
func (c *WebSocketConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
