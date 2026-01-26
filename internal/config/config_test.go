package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Create a temporary config file
	content := `
mode: client
server:
  address: "localhost:8080"
ports:
  - local: 3306
    remote: 3306
transport:
  protocol: http
  multiplex: true
  packet_size: 4096
  compress: true
  buffer_size: 65536
resilience:
  health_check_interval: 5s
  reconnect_min_delay: 1s
  reconnect_max_delay: 30s
  connection_timeout: 10s
  request_timeout: 30s
logging:
  level: info
  format: text
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Mode != "client" {
		t.Errorf("expected mode 'client', got '%s'", cfg.Mode)
	}

	if cfg.Server.Address != "localhost:8080" {
		t.Errorf("expected server address 'localhost:8080', got '%s'", cfg.Server.Address)
	}

	if len(cfg.Ports) != 1 {
		t.Errorf("expected 1 port mapping, got %d", len(cfg.Ports))
	}

	if cfg.Ports[0].Local != 3306 || cfg.Ports[0].Remote != 3306 {
		t.Errorf("unexpected port mapping: %+v", cfg.Ports[0])
	}
}

func TestDefaultClientConfig(t *testing.T) {
	cfg := DefaultClientConfig()

	if cfg.Mode != "client" {
		t.Errorf("expected mode 'client', got '%s'", cfg.Mode)
	}

	if cfg.Transport.Protocol != "http" {
		t.Errorf("expected protocol 'http', got '%s'", cfg.Transport.Protocol)
	}

	if cfg.Transport.PacketSize != 4096 {
		t.Errorf("expected packet size 4096, got %d", cfg.Transport.PacketSize)
	}

	if cfg.Resilience.HealthCheckInterval != 5*time.Second {
		t.Errorf("expected health check interval 5s, got %s", cfg.Resilience.HealthCheckInterval)
	}
}

func TestDefaultServerConfig(t *testing.T) {
	cfg := DefaultServerConfig()

	if cfg.Mode != "server" {
		t.Errorf("expected mode 'server', got '%s'", cfg.Mode)
	}

	if cfg.Listen.Address != "0.0.0.0:443" {
		t.Errorf("expected listen address '0.0.0.0:443', got '%s'", cfg.Listen.Address)
	}

	if cfg.Listen.Path != "/tunnel" {
		t.Errorf("expected path '/tunnel', got '%s'", cfg.Listen.Path)
	}

	if len(cfg.Security.AllowedPorts) != 3 {
		t.Errorf("expected 3 allowed ports, got %d", len(cfg.Security.AllowedPorts))
	}
}
