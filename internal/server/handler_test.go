package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sahmadiut/cloudtunnel/internal/config"
	"github.com/sahmadiut/cloudtunnel/internal/logger"
)

func TestHandler_AuthCheck(t *testing.T) {
	log := logger.NewDefault()
	cfg := &config.Config{
		Security: config.SecurityConfig{
			AuthToken: "test-token",
		},
		Transport: config.TransportConfig{
			BufferSize: 65536,
		},
	}

	handler := NewHandler(cfg, log)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{
			name:       "no auth header",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong token",
			authHeader: "Bearer wrong-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "correct token",
			authHeader: "Bearer test-token",
			wantStatus: http.StatusOK, // GET returns OK
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tunnel", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}
		})
	}
}

func TestHandler_NoAuth(t *testing.T) {
	log := logger.NewDefault()
	cfg := &config.Config{
		Security: config.SecurityConfig{
			AuthToken: "", // No auth required
		},
		Transport: config.TransportConfig{
			BufferSize: 65536,
		},
	}

	handler := NewHandler(cfg, log)

	req := httptest.NewRequest(http.MethodGet, "/tunnel", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	log := logger.NewDefault()
	cfg := &config.Config{
		Transport: config.TransportConfig{
			BufferSize: 65536,
		},
	}

	handler := NewHandler(cfg, log)

	methods := []string{http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/tunnel", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status %d for %s, got %d", http.StatusMethodNotAllowed, method, rr.Code)
			}
		})
	}
}

func TestParseConnectHeader(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		want    int
		wantErr bool
	}{
		{
			name:    "valid header",
			header:  "CONNECT 8080",
			want:    8080,
			wantErr: false,
		},
		{
			name:    "valid header with newline",
			header:  "CONNECT 3306\n",
			want:    3306,
			wantErr: false,
		},
		{
			name:    "invalid format",
			header:  "PORT 8080",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid port",
			header:  "CONNECT abc",
			want:    0,
			wantErr: true,
		},
		{
			name:    "port out of range",
			header:  "CONNECT 70000",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConnectHeader(tt.header)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseConnectHeader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseConnectHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPortAllowed(t *testing.T) {
	log := logger.NewDefault()

	tests := []struct {
		name         string
		allowedPorts []int
		port         int
		want         bool
	}{
		{
			name:         "empty list allows all",
			allowedPorts: []int{},
			port:         8080,
			want:         true,
		},
		{
			name:         "port in list",
			allowedPorts: []int{3306, 6379, 8080},
			port:         8080,
			want:         true,
		},
		{
			name:         "port not in list",
			allowedPorts: []int{3306, 6379},
			port:         8080,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Security: config.SecurityConfig{
					AllowedPorts: tt.allowedPorts,
				},
				Transport: config.TransportConfig{
					BufferSize: 65536,
				},
			}
			handler := NewHandler(cfg, log)
			got := handler.isPortAllowed(tt.port)
			if got != tt.want {
				t.Errorf("isPortAllowed(%d) = %v, want %v", tt.port, got, tt.want)
			}
		})
	}
}

func TestHandler_POST_BadRequest(t *testing.T) {
	log := logger.NewDefault()
	cfg := &config.Config{
		Security: config.SecurityConfig{
			AllowedPorts: []int{8080},
		},
		Transport: config.TransportConfig{
			BufferSize: 65536,
		},
	}

	handler := NewHandler(cfg, log)

	// POST without proper CONNECT header
	req := httptest.NewRequest(http.MethodPost, "/tunnel", strings.NewReader("invalid data"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
