// Package transport provides connection abstractions for tunnel data transfer
package transport

import (
	"io"
	"net"
	"time"
)

// Conn is an interface that represents a tunnel connection
type Conn interface {
	io.ReadWriteCloser

	// LocalAddr returns the local network address
	LocalAddr() net.Addr

	// RemoteAddr returns the remote network address
	RemoteAddr() net.Addr

	// SetDeadline sets the read and write deadlines
	SetDeadline(t time.Time) error

	// SetReadDeadline sets the read deadline
	SetReadDeadline(t time.Time) error

	// SetWriteDeadline sets the write deadline
	SetWriteDeadline(t time.Time) error
}

// Transport is an interface for establishing tunnel connections
type Transport interface {
	// Dial establishes a new connection to the tunnel server
	Dial() (Conn, error)

	// Close closes the transport
	Close() error
}

// Listener is an interface for accepting tunnel connections
type Listener interface {
	// Accept waits for and returns the next connection
	Accept() (Conn, error)

	// Close closes the listener
	Close() error

	// Addr returns the listener's network address
	Addr() net.Addr
}

// connWrapper wraps a net.Conn to implement Conn interface
type connWrapper struct {
	net.Conn
}

// WrapConn wraps a net.Conn into a Conn
func WrapConn(conn net.Conn) Conn {
	return &connWrapper{Conn: conn}
}

// tunnelAddr implements net.Addr for tunnel connections
type tunnelAddr struct {
	network string
	address string
}

func (a *tunnelAddr) Network() string {
	return a.network
}

func (a *tunnelAddr) String() string {
	return a.address
}

// NewAddr creates a new tunnel address
func NewAddr(network, address string) net.Addr {
	return &tunnelAddr{network: network, address: address}
}
