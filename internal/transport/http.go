// Package transport provides HTTP-based tunnel transport
package transport

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sahmadiut/cloudtunnel/internal/config"
)

// HTTPTransport implements Transport using HTTP/HTTPS
type HTTPTransport struct {
	cfg        *config.Config
	serverURL  string
	dialer     *net.Dialer
	tlsConfig  *tls.Config
	mu         sync.Mutex
	closed     bool
	resolveIP  string
	targetHost string
	serverAddr string
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(cfg *config.Config) (*HTTPTransport, error) {
	scheme := "http"
	if cfg.TLS.Enabled {
		scheme = "https"
	}

	serverURL := fmt.Sprintf("%s://%s%s", scheme, cfg.Server.Address, "/tunnel")

	// Parse the host for SNI
	host, _, err := net.SplitHostPort(cfg.Server.Address)
	if err != nil {
		host = cfg.Server.Address
	}

	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
	}

	t := &HTTPTransport{
		cfg:        cfg,
		serverURL:  serverURL,
		serverAddr: cfg.Server.Address,
		resolveIP:  cfg.Server.ResolveIP,
		targetHost: host,
		dialer: &net.Dialer{
			Timeout:   cfg.Resilience.ConnectionTimeout,
			KeepAlive: 30 * time.Second,
		},
		tlsConfig: tlsConfig,
	}

	return t, nil
}

// Dial establishes a new HTTP tunnel connection
func (t *HTTPTransport) Dial() (Conn, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, errors.New("transport is closed")
	}
	t.mu.Unlock()

	// Determine the address to connect to
	addr := t.serverAddr
	if t.resolveIP != "" {
		_, port, err := net.SplitHostPort(t.serverAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse server address: %w", err)
		}
		addr = net.JoinHostPort(t.resolveIP, port)
	}

	// Establish raw TCP connection
	ctx, cancel := context.WithTimeout(context.Background(), t.cfg.Resilience.ConnectionTimeout)
	defer cancel()

	conn, err := t.dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	// Upgrade to TLS if enabled
	if t.cfg.TLS.Enabled {
		tlsConn := tls.Client(conn, t.tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, fmt.Errorf("TLS handshake failed: %w", err)
		}
		conn = tlsConn
	}

	return NewHTTPStreamConn(conn, t.cfg, t.targetHost), nil
}

// Close closes the transport
func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	return nil
}

// HTTPStreamConn implements a persistent streaming connection over HTTP
// Uses HTTP CONNECT-style tunneling for bidirectional TCP streaming
type HTTPStreamConn struct {
	conn       net.Conn
	cfg        *config.Config
	host       string
	reader     *bufio.Reader
	mu         sync.Mutex
	closed     bool
	ready      bool // Connection is ready for bidirectional streaming
	localAddr  net.Addr
	remoteAddr net.Addr
}

// NewHTTPStreamConn creates a new HTTP streaming connection
func NewHTTPStreamConn(conn net.Conn, cfg *config.Config, host string) *HTTPStreamConn {
	return &HTTPStreamConn{
		conn:       conn,
		cfg:        cfg,
		host:       host,
		reader:     bufio.NewReader(conn),
		localAddr:  conn.LocalAddr(),
		remoteAddr: conn.RemoteAddr(),
	}
}

// setupTunnel sends the tunnel setup request and waits for confirmation
func (c *HTTPStreamConn) setupTunnel(port int) error {
	if c.ready {
		return nil
	}

	// Validate port
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}

	// Send HTTP POST request with tunnel port in header
	req := fmt.Sprintf("POST /tunnel HTTP/1.1\r\n"+
		"Host: %s\r\n"+
		"Content-Type: application/octet-stream\r\n"+
		"X-Tunnel-Port: %d\r\n", c.host, port)

	// Add auth token if configured
	if c.cfg.Security.AuthToken != "" {
		req += fmt.Sprintf("Authorization: Bearer %s\r\n", c.cfg.Security.AuthToken)
	}

	req += "\r\n"

	// Send the request
	if _, err := c.conn.Write([]byte(req)); err != nil {
		return fmt.Errorf("failed to send tunnel request: %w", err)
	}

	// Read and parse HTTP response
	resp, err := http.ReadResponse(c.reader, nil)
	if err != nil {
		return fmt.Errorf("failed to read tunnel response: %w", err)
	}

	// Check for authentication errors
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("authentication failed: invalid token (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tunnel setup failed: HTTP %d", resp.StatusCode)
	}

	c.ready = true
	return nil
}

// Write writes data to the connection
// First write must contain "CONNECT <port>\n" to establish the tunnel
// This header is parsed locally and converted to HTTP X-Tunnel-Port header
func (c *HTTPStreamConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, errors.New("connection closed")
	}

	// If not ready, first data should be "CONNECT <port>\n"
	if !c.ready {
		// Parse the CONNECT header from the data to extract the port
		port := 0
		n, err := fmt.Sscanf(string(p), "CONNECT %d\n", &port)
		if err != nil || n != 1 || port <= 0 {
			return 0, fmt.Errorf("first write must be 'CONNECT <port>\\n', got: %s", string(p))
		}

		// Setup the tunnel with the specified port (sends HTTP request)
		if err := c.setupTunnel(port); err != nil {
			return 0, err
		}

		// The CONNECT header is consumed and converted to HTTP protocol
		return len(p), nil
	}

	// Write data directly to the connection
	return c.conn.Write(p)
}

// Read reads data from the connection
func (c *HTTPStreamConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, io.EOF
	}

	// Must be ready before reading
	if !c.ready {
		return 0, errors.New("tunnel not ready: must write CONNECT header first")
	}

	// Read data directly from the connection
	return c.reader.Read(p)
}

// Close closes the connection
func (c *HTTPStreamConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.conn.Close()
}

// LocalAddr returns the local network address
func (c *HTTPStreamConn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr returns the remote network address
func (c *HTTPStreamConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// SetDeadline sets the read and write deadlines
func (c *HTTPStreamConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

// SetReadDeadline sets the read deadline
func (c *HTTPStreamConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline
func (c *HTTPStreamConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
