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

// Dial establishes a new HTTP tunnel connection with persistent streaming
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
type HTTPStreamConn struct {
	conn           net.Conn
	cfg            *config.Config
	host           string
	reader         *bufio.Reader
	mu             sync.Mutex
	closed         bool
	headerSent     bool
	responseRead   bool
	localAddr      net.Addr
	remoteAddr     net.Addr
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

// sendHTTPHeader sends the initial HTTP POST request header
func (c *HTTPStreamConn) sendHTTPHeader() error {
	if c.headerSent {
		return nil
	}

	// Build HTTP POST request
	req := fmt.Sprintf("POST /tunnel HTTP/1.1\r\n"+
		"Host: %s\r\n"+
		"Content-Type: application/octet-stream\r\n"+
		"Connection: keep-alive\r\n", c.host)

	// Add auth token if configured
	if c.cfg.Security.AuthToken != "" {
		req += fmt.Sprintf("Authorization: Bearer %s\r\n", c.cfg.Security.AuthToken)
	}

	req += "\r\n"

	_, err := c.conn.Write([]byte(req))
	if err != nil {
		return fmt.Errorf("failed to send HTTP header: %w", err)
	}

	c.headerSent = true
	return nil
}

// Write writes data to the connection
func (c *HTTPStreamConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, errors.New("connection closed")
	}

	// Send HTTP header on first write
	if err := c.sendHTTPHeader(); err != nil {
		return 0, err
	}

	// Write tunnel data directly to the HTTP POST body stream
	n, err := c.conn.Write(p)
	if err != nil {
		return n, err
	}

	return n, nil
}

// readHTTPResponse reads and validates the HTTP response headers
func (c *HTTPStreamConn) readHTTPResponse() error {
	if c.responseRead {
		return nil
	}

	// Read HTTP response
	resp, err := http.ReadResponse(c.reader, nil)
	if err != nil {
		return fmt.Errorf("failed to read HTTP response: %w", err)
	}
	
	c.responseRead = true

	// Check for authentication errors
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("authentication failed: invalid token (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// Read reads data from the connection
func (c *HTTPStreamConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, io.EOF
	}

	// Read HTTP response header once after we've sent our request
	if c.headerSent && !c.responseRead {
		if err := c.readHTTPResponse(); err != nil {
			return 0, err
		}
	}

	// Read data directly from the connection (after response headers)
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
