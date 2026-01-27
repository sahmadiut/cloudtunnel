// Package server provides HTTP request handling for the tunnel server
package server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/sahmadiut/cloudtunnel/internal/config"
	"github.com/sahmadiut/cloudtunnel/internal/logger"
)

// Handler handles tunnel HTTP requests
type Handler struct {
	cfg      *config.Config
	log      *logger.Logger
	upgrader websocket.Upgrader
	mu       sync.RWMutex
	conns    map[string]net.Conn
}

// NewHandler creates a new tunnel handler
func NewHandler(cfg *config.Config, log *logger.Logger) *Handler {
	return &Handler{
		cfg: cfg,
		log: log,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  cfg.Transport.BufferSize,
			WriteBufferSize: cfg.Transport.BufferSize,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
		conns: make(map[string]net.Conn),
	}
}

// ServeHTTP handles HTTP tunnel requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check authentication
	if h.cfg.Security.AuthToken != "" {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + h.cfg.Security.AuthToken
		if auth != expected {
			h.log.Warn().
				Str("remote_addr", r.RemoteAddr).
				Msg("unauthorized request")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Check if it's a WebSocket upgrade
	if r.Header.Get("Upgrade") == "websocket" {
		h.handleWebSocket(w, r)
		return
	}

	// Handle HTTP long-polling
	h.handleHTTP(w, r)
}

// handleWebSocket handles WebSocket tunnel connections
func (h *Handler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error().
			Err(err).
			Str("remote_addr", r.RemoteAddr).
			Msg("websocket upgrade failed")
		return
	}
	defer conn.Close()

	h.log.Debug().
		Str("remote_addr", r.RemoteAddr).
		Msg("websocket connection established")

	// Read connection header
	_, msg, err := conn.ReadMessage()
	if err != nil {
		h.log.Error().
			Err(err).
			Msg("failed to read connection header")
		return
	}

	// Parse CONNECT header
	port, err := parseConnectHeader(string(msg))
	if err != nil {
		h.log.Error().
			Err(err).
			Str("header", string(msg)).
			Msg("invalid connection header")
		return
	}

	// Check if port is allowed
	if !h.isPortAllowed(port) {
		h.log.Warn().
			Int("port", port).
			Msg("port not allowed")
		return
	}

	// Connect to target
	target, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		h.log.Error().
			Err(err).
			Int("port", port).
			Msg("failed to connect to target")
		return
	}
	defer target.Close()

	h.log.Debug().
		Int("port", port).
		Str("remote_addr", r.RemoteAddr).
		Msg("connected to target")

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	// WebSocket -> Target
	go func() {
		defer wg.Done()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if _, err := target.Write(data); err != nil {
				return
			}
		}
	}()

	// Target -> WebSocket
	go func() {
		defer wg.Done()
		buf := make([]byte, h.cfg.Transport.PacketSize)
		for {
			n, err := target.Read(buf)
			if err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	wg.Wait()
}

// handleHTTP handles HTTP long-polling tunnel connections
func (h *Handler) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// For HTTP transport, we use request/response pairs
	// POST sends data to server, GET receives data from server

	switch r.Method {
	case http.MethodPost:
		h.handleHTTPPost(w, r)
	case http.MethodGet:
		h.handleHTTPGet(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleHTTPPost handles POST requests (client -> server data)
func (h *Handler) handleHTTPPost(w http.ResponseWriter, r *http.Request) {
	// Read the body
	reader := bufio.NewReader(r.Body)
	defer r.Body.Close()

	// First line should be CONNECT header
	line, err := reader.ReadString('\n')
	if err != nil {
		h.log.Error().
			Err(err).
			Msg("failed to read connection header")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Parse CONNECT header
	port, err := parseConnectHeader(line)
	if err != nil {
		h.log.Error().
			Err(err).
			Str("header", line).
			Msg("invalid connection header")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Check if port is allowed
	if !h.isPortAllowed(port) {
		h.log.Warn().
			Int("port", port).
			Msg("port not allowed")
		http.Error(w, "Port not allowed", http.StatusForbidden)
		return
	}

	// Connect to target
	target, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		h.log.Error().
			Err(err).
			Int("port", port).
			Msg("failed to connect to target")
		http.Error(w, "Connection failed", http.StatusBadGateway)
		return
	}
	defer target.Close()

	h.log.Debug().
		Int("port", port).
		Str("remote_addr", r.RemoteAddr).
		Msg("connected to target via HTTP")

	// Try to hijack the connection for persistent streaming
	hijacker, ok := w.(http.Hijacker)
	if ok {
		// Hijack the connection for true bidirectional streaming
		clientConn, buf, err := hijacker.Hijack()
		if err != nil {
			h.log.Error().
				Err(err).
				Msg("failed to hijack connection")
			// Fall back to non-hijacked mode
			h.handleHTTPPostNonHijacked(w, r, reader, target)
			return
		}
		defer clientConn.Close()

		// Send HTTP 200 response manually
		if _, err := clientConn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\n\r\n")); err != nil {
			h.log.Error().
				Err(err).
				Msg("failed to write HTTP response")
			return
		}

		h.log.Debug().
			Int("port", port).
			Str("remote_addr", r.RemoteAddr).
			Msg("connection hijacked for persistent streaming")

		// Bidirectional copy with proper stream handling
		var wg sync.WaitGroup
		wg.Add(2)

		// Client -> Target (drain buffered data first, then read from connection)
		go func() {
			defer wg.Done()
			// First drain any buffered data from the reader
			if buf.Reader.Buffered() > 0 {
				if _, err := io.CopyN(target, buf, int64(buf.Reader.Buffered())); err != nil {
					h.log.Debug().Err(err).Msg("error draining buffered data")
				}
			}
			io.Copy(target, clientConn)
			// Try to half-close the write side if it's a TCP connection
			if tcpConn, ok := target.(*net.TCPConn); ok {
				tcpConn.CloseWrite()
			}
		}()

		// Target -> Client
		go func() {
			defer wg.Done()
			io.Copy(clientConn, target)
		}()

		wg.Wait()
		return
	}

	// Fall back to non-hijacked mode
	h.handleHTTPPostNonHijacked(w, r, reader, target)
}

// handleHTTPPostNonHijacked handles POST without connection hijacking
func (h *Handler) handleHTTPPostNonHijacked(w http.ResponseWriter, r *http.Request, reader *bufio.Reader, target net.Conn) {
	// Copy remaining data to target
	go func() {
		io.Copy(target, reader)
	}()

	// Return response with data from target
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)

	// Flush immediately
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	io.Copy(w, target)
}

// handleHTTPGet handles GET requests (server -> client data)
func (h *Handler) handleHTTPGet(w http.ResponseWriter, r *http.Request) {
	// For simple GET, just return OK to indicate server is alive
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
}

// parseConnectHeader parses a CONNECT header and returns the port
func parseConnectHeader(header string) (int, error) {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(header, "CONNECT ") {
		return 0, fmt.Errorf("invalid header format")
	}

	portStr := strings.TrimPrefix(header, "CONNECT ")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port: %w", err)
	}

	if port <= 0 || port > 65535 {
		return 0, fmt.Errorf("port out of range: %d", port)
	}

	return port, nil
}

// isPortAllowed checks if the port is in the allowed list
func (h *Handler) isPortAllowed(port int) bool {
	// If no allowed ports configured, allow all
	if len(h.cfg.Security.AllowedPorts) == 0 {
		return true
	}

	for _, p := range h.cfg.Security.AllowedPorts {
		if p == port {
			return true
		}
	}

	return false
}
