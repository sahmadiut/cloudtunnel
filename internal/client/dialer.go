// Package client provides the client-side tunnel implementation
package client

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/sahmadiut/cloudtunnel/internal/config"
)

// Dialer provides custom dialing with resolve_ip support
type Dialer struct {
	cfg       *config.Config
	tlsConfig *tls.Config
	netDialer *net.Dialer
}

// NewDialer creates a new dialer with the given configuration
func NewDialer(cfg *config.Config) *Dialer {
	// Parse the host for SNI
	host, _, err := net.SplitHostPort(cfg.Server.Address)
	if err != nil {
		host = cfg.Server.Address
	}

	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
	}

	return &Dialer{
		cfg:       cfg,
		tlsConfig: tlsConfig,
		netDialer: &net.Dialer{
			Timeout:   cfg.Resilience.ConnectionTimeout,
			KeepAlive: 30 * time.Second,
		},
	}
}

// Dial establishes a connection to the server
func (d *Dialer) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	// Parse the original address
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	// Use resolve_ip if specified
	dialAddr := address
	if d.cfg.Server.ResolveIP != "" {
		dialAddr = net.JoinHostPort(d.cfg.Server.ResolveIP, port)
	}

	// Dial the connection
	conn, err := d.netDialer.DialContext(ctx, network, dialAddr)
	if err != nil {
		return nil, err
	}

	// Wrap with TLS if enabled
	if d.cfg.TLS.Enabled {
		tlsConfig := d.tlsConfig.Clone()
		tlsConfig.ServerName = host

		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, err
		}
		return tlsConn, nil
	}

	return conn, nil
}

// DialTCP establishes a TCP connection with custom dialing
func (d *Dialer) DialTCP(ctx context.Context) (net.Conn, error) {
	return d.Dial(ctx, "tcp", d.cfg.Server.Address)
}
