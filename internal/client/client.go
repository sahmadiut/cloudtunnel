// Package client provides the client-side tunnel implementation
package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/sahmadiut/cloudtunnel/internal/config"
	"github.com/sahmadiut/cloudtunnel/internal/health"
	"github.com/sahmadiut/cloudtunnel/internal/logger"
	"github.com/sahmadiut/cloudtunnel/internal/transport"
)

// Client represents a tunnel client
type Client struct {
	cfg       *config.Config
	log       *logger.Logger
	transport transport.Transport
	forwarder *PortForwarder
	checker   *health.Checker
	reconnect *health.Reconnector

	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new tunnel client
func New(cfg *config.Config, log *logger.Logger) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		cfg:    cfg,
		log:    log,
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize transport based on protocol
	var tr transport.Transport
	var err error

	switch cfg.Transport.Protocol {
	case "http":
		tr, err = transport.NewHTTPTransport(cfg)
	case "websocket":
		tr, err = transport.NewWebSocketTransport(cfg)
	default:
		return nil, fmt.Errorf("unsupported transport protocol: %s", cfg.Transport.Protocol)
	}

	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	c.transport = tr

	// Create port forwarder
	c.forwarder = NewPortForwarder(cfg, tr, log)

	// Create reconnector
	c.reconnect = health.NewReconnector(
		c.connect,
		health.ReconnectorConfig{
			MinDelay: cfg.Resilience.ReconnectMinDelay,
			MaxDelay: cfg.Resilience.ReconnectMaxDelay,
		},
		log,
	)

	return c, nil
}

// Start starts the tunnel client
func (c *Client) Start() error {
	c.log.Info().
		Str("server", c.cfg.Server.Address).
		Str("protocol", c.cfg.Transport.Protocol).
		Bool("tls", c.cfg.TLS.Enabled).
		Bool("multiplex", c.cfg.Transport.Multiplex).
		Msg("starting tunnel client")

	// Start port forwarder
	if err := c.forwarder.Start(); err != nil {
		return fmt.Errorf("failed to start port forwarder: %w", err)
	}

	// Initialize and start health checker
	c.checker = health.NewChecker(c, health.CheckerConfig{
		Interval: c.cfg.Resilience.HealthCheckInterval,
		Timeout:  c.cfg.Resilience.ConnectionTimeout,
		OnUnhealthy: func() {
			c.log.Warn().Msg("server connection unhealthy, attempting reconnection")
			c.reconnect.ReconnectAsync(func(err error) {
				if err != nil {
					c.log.Error().Err(err).Msg("reconnection failed")
				}
			})
		},
		OnRecovered: func() {
			c.log.Info().Msg("server connection recovered")
		},
	}, c.log)
	c.checker.Start()
	c.log.Debug().
		Dur("interval", c.cfg.Resilience.HealthCheckInterval).
		Msg("health checker started")

	c.log.Info().
		Int("ports", len(c.cfg.Ports)).
		Msg("tunnel client started")

	return nil
}

// Stop stops the tunnel client
func (c *Client) Stop() {
	c.log.Info().Msg("stopping tunnel client")

	c.cancel()

	if c.checker != nil {
		c.checker.Stop()
	}

	if c.forwarder != nil {
		c.forwarder.Stop()
	}

	if c.reconnect != nil {
		c.reconnect.Stop()
	}

	if c.transport != nil {
		c.transport.Close()
	}

	c.wg.Wait()

	c.log.Info().Msg("tunnel client stopped")
}

// connect establishes a connection to the server
func (c *Client) connect(ctx context.Context) error {
	// Test connection by dialing
	conn, err := c.transport.Dial()
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// Ping implements health.Pinger for health checking
func (c *Client) Ping(ctx context.Context) error {
	// Create a simple HTTP GET request to check server health
	scheme := "http"
	if c.cfg.TLS.Enabled {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/tunnel", scheme, c.cfg.Server.Address)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	if c.cfg.Security.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Security.AuthToken)
	}

	// Create a simple client with custom dialer for resolve_ip support
	dialer := &net.Dialer{
		Timeout: c.cfg.Resilience.ConnectionTimeout,
	}
	client := &http.Client{
		Timeout: c.cfg.Resilience.ConnectionTimeout,
		Transport: &http.Transport{
			DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
				if c.cfg.Server.ResolveIP != "" {
					_, port, err := net.SplitHostPort(addr)
					if err != nil {
						return nil, err
					}
					addr = net.JoinHostPort(c.cfg.Server.ResolveIP, port)
				}
				return dialer.DialContext(dialCtx, network, addr)
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: c.cfg.TLS.InsecureSkipVerify,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("authentication failed: invalid token (HTTP %d)", resp.StatusCode)
	}

	return nil
}

// Wait blocks until the client is stopped
func (c *Client) Wait() {
	<-c.ctx.Done()
	c.wg.Wait()
}
