// Package client provides multi-port forwarding management
package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/sahmadiut/cloudtunnel/internal/config"
	"github.com/sahmadiut/cloudtunnel/internal/logger"
	"github.com/sahmadiut/cloudtunnel/internal/transport"
)

// PortForwarder manages port forwarding for multiple port mappings
type PortForwarder struct {
	cfg       *config.Config
	log       *logger.Logger
	transport transport.Transport

	mu        sync.Mutex
	listeners map[int]net.Listener
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewPortForwarder creates a new port forwarder
func NewPortForwarder(cfg *config.Config, tr transport.Transport, log *logger.Logger) *PortForwarder {
	ctx, cancel := context.WithCancel(context.Background())

	return &PortForwarder{
		cfg:       cfg,
		log:       log,
		transport: tr,
		listeners: make(map[int]net.Listener),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts listening on all configured ports
func (pf *PortForwarder) Start() error {
	for _, pm := range pf.cfg.Ports {
		if err := pf.startPort(pm); err != nil {
			pf.Stop()
			return err
		}
	}

	return nil
}

// startPort starts listening on a single port
func (pf *PortForwarder) startPort(pm config.PortMapping) error {
	addr := fmt.Sprintf("127.0.0.1:%d", pm.Local)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", pm.Local, err)
	}

	pf.mu.Lock()
	pf.listeners[pm.Local] = listener
	pf.mu.Unlock()

	pf.log.Info().
		Int("local_port", pm.Local).
		Int("remote_port", pm.Remote).
		Str("address", addr).
		Msg("listening for connections")

	pf.wg.Add(1)
	go pf.acceptLoop(listener, pm)

	return nil
}

// acceptLoop accepts connections on a listener and forwards them
func (pf *PortForwarder) acceptLoop(listener net.Listener, pm config.PortMapping) {
	defer pf.wg.Done()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-pf.ctx.Done():
				return
			default:
				pf.log.Error().
					Err(err).
					Int("port", pm.Local).
					Msg("failed to accept connection")
				continue
			}
		}

		pf.wg.Add(1)
		go pf.handleConnection(conn, pm)
	}
}

// handleConnection handles a single forwarded connection
func (pf *PortForwarder) handleConnection(localConn net.Conn, pm config.PortMapping) {
	defer pf.wg.Done()
	defer localConn.Close()

	pf.log.Debug().
		Int("local_port", pm.Local).
		Int("remote_port", pm.Remote).
		Str("remote_addr", localConn.RemoteAddr().String()).
		Msg("new connection")

	// Establish tunnel connection
	tunnelConn, err := pf.transport.Dial()
	if err != nil {
		pf.log.Error().
			Err(err).
			Int("port", pm.Local).
			Msg("failed to dial tunnel")
		return
	}
	defer tunnelConn.Close()

	// Send connection header with remote port
	header := fmt.Sprintf("CONNECT %d\n", pm.Remote)
	if _, err := tunnelConn.Write([]byte(header)); err != nil {
		pf.log.Error().
			Err(err).
			Int("port", pm.Remote).
			Msg("failed to send connection header")
		return
	}

	// Bidirectional copy
	ctx, cancel := context.WithCancel(pf.ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer cancel()
		_, err := io.Copy(tunnelConn, localConn)
		if err != nil && ctx.Err() == nil {
			pf.log.Debug().
				Err(err).
				Int("port", pm.Local).
				Msg("local->tunnel copy error")
		}
	}()

	go func() {
		defer wg.Done()
		defer cancel()
		_, err := io.Copy(localConn, tunnelConn)
		if err != nil && ctx.Err() == nil {
			pf.log.Debug().
				Err(err).
				Int("port", pm.Local).
				Msg("tunnel->local copy error")
		}
	}()

	wg.Wait()

	pf.log.Debug().
		Int("local_port", pm.Local).
		Int("remote_port", pm.Remote).
		Msg("connection closed")
}

// Stop stops all port forwarding
func (pf *PortForwarder) Stop() {
	pf.cancel()

	pf.mu.Lock()
	for port, listener := range pf.listeners {
		listener.Close()
		delete(pf.listeners, port)
	}
	pf.mu.Unlock()

	pf.wg.Wait()
}

// Ports returns the list of active port mappings
func (pf *PortForwarder) Ports() []config.PortMapping {
	return pf.cfg.Ports
}
