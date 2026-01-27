// Package transport provides HTTP-based tunnel transport
package transport

import (
	"bytes"
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
	client     *http.Client
	serverURL  string
	dialer     *net.Dialer
	tlsConfig  *tls.Config
	mu         sync.Mutex
	closed     bool
	resolveIP  string
	targetHost string
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
		resolveIP:  cfg.Server.ResolveIP,
		targetHost: host,
		dialer: &net.Dialer{
			Timeout:   cfg.Resilience.ConnectionTimeout,
			KeepAlive: 30 * time.Second,
		},
		tlsConfig: tlsConfig,
	}

	// Create HTTP client with custom dialer
	transport := &http.Transport{
		DialContext:         t.dialContext,
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	t.client = &http.Client{
		Transport: transport,
		Timeout:   cfg.Resilience.RequestTimeout,
	}

	return t, nil
}

// dialContext is a custom dial function that supports resolve_ip
func (t *HTTPTransport) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// If resolve_ip is set, connect to the specified IP instead of resolving
	if t.resolveIP != "" {
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		addr = net.JoinHostPort(t.resolveIP, port)
	}

	return t.dialer.DialContext(ctx, network, addr)
}

// Dial establishes a new HTTP tunnel connection
func (t *HTTPTransport) Dial() (Conn, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, errors.New("transport is closed")
	}
	t.mu.Unlock()

	return NewHTTPConn(t.client, t.serverURL, t.cfg), nil
}

// Close closes the transport
func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	t.client.CloseIdleConnections()
	return nil
}

// HTTPConn implements Conn over HTTP
type HTTPConn struct {
	client     *http.Client
	serverURL  string
	cfg        *config.Config
	readBuf    bytes.Buffer
	writeBuf   bytes.Buffer
	mu         sync.Mutex
	closed     bool
	localAddr  net.Addr
	remoteAddr net.Addr
}

// NewHTTPConn creates a new HTTP connection
func NewHTTPConn(client *http.Client, serverURL string, cfg *config.Config) *HTTPConn {
	return &HTTPConn{
		client:     client,
		serverURL:  serverURL,
		cfg:        cfg,
		localAddr:  NewAddr("tcp", "local"),
		remoteAddr: NewAddr("tcp", serverURL),
	}
}

// Read reads data from the HTTP connection
func (c *HTTPConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, io.EOF
	}

	// If we have buffered data, return it
	if c.readBuf.Len() > 0 {
		return c.readBuf.Read(p)
	}

	// Make a GET request to receive data
	req, err := http.NewRequest("GET", c.serverURL, nil)
	if err != nil {
		return 0, err
	}

	// Add auth token if configured
	if c.cfg.Security.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Security.AuthToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Check for authentication errors with clear logging
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return 0, fmt.Errorf("authentication failed: invalid token (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body into buffer
	_, err = io.Copy(&c.readBuf, resp.Body)
	if err != nil {
		return 0, err
	}

	return c.readBuf.Read(p)
}

// Write writes data to the HTTP connection
func (c *HTTPConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, errors.New("connection closed")
	}

	// Make a POST request to send data
	req, err := http.NewRequest("POST", c.serverURL, bytes.NewReader(p))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	// Add auth token if configured
	if c.cfg.Security.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Security.AuthToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Check for authentication errors with clear logging
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return 0, fmt.Errorf("authentication failed: invalid token (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return len(p), nil
}

// Close closes the HTTP connection
func (c *HTTPConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true
	return nil
}

// LocalAddr returns the local network address
func (c *HTTPConn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr returns the remote network address
func (c *HTTPConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// SetDeadline sets the read and write deadlines
func (c *HTTPConn) SetDeadline(t time.Time) error {
	return nil // HTTP doesn't support per-connection deadlines directly
}

// SetReadDeadline sets the read deadline
func (c *HTTPConn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline sets the write deadline
func (c *HTTPConn) SetWriteDeadline(t time.Time) error {
	return nil
}
