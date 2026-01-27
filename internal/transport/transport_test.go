package transport

import (
	"testing"
)

func TestNewAddr(t *testing.T) {
	addr := NewAddr("tcp", "127.0.0.1:8080")

	if addr.Network() != "tcp" {
		t.Errorf("expected network 'tcp', got '%s'", addr.Network())
	}

	if addr.String() != "127.0.0.1:8080" {
		t.Errorf("expected address '127.0.0.1:8080', got '%s'", addr.String())
	}
}

func TestDefaultMultiplexConfig(t *testing.T) {
	cfg := DefaultMultiplexConfig()

	if cfg.AcceptBacklog != 256 {
		t.Errorf("expected AcceptBacklog 256, got %d", cfg.AcceptBacklog)
	}

	if !cfg.EnableKeepAlive {
		t.Error("expected EnableKeepAlive to be true")
	}
}
