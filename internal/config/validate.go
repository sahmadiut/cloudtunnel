// Package config provides configuration validation
package config

import (
	"errors"
	"fmt"
	"net"
)

// Validate validates the configuration
func Validate(cfg *Config) error {
	if cfg == nil {
		return errors.New("configuration is nil")
	}

	switch cfg.Mode {
	case "client":
		return validateClientConfig(cfg)
	case "server":
		return validateServerConfig(cfg)
	default:
		return fmt.Errorf("invalid mode: %s, must be 'client' or 'server'", cfg.Mode)
	}
}

// validateClientConfig validates client-specific configuration
func validateClientConfig(cfg *Config) error {
	// Validate server address
	if cfg.Server.Address == "" {
		return errors.New("server address is required for client mode")
	}

	// Validate server address format
	if _, _, err := net.SplitHostPort(cfg.Server.Address); err != nil {
		return fmt.Errorf("invalid server address format: %w", err)
	}

	// Validate resolve_ip if specified
	if cfg.Server.ResolveIP != "" {
		if ip := net.ParseIP(cfg.Server.ResolveIP); ip == nil {
			return fmt.Errorf("invalid resolve_ip: %s", cfg.Server.ResolveIP)
		}
	}

	// Validate port mappings
	if len(cfg.Ports) == 0 {
		return errors.New("at least one port mapping is required")
	}

	for i, pm := range cfg.Ports {
		if pm.Local <= 0 || pm.Local > 65535 {
			return fmt.Errorf("invalid local port in mapping %d: %d", i, pm.Local)
		}
		if pm.Remote <= 0 || pm.Remote > 65535 {
			return fmt.Errorf("invalid remote port in mapping %d: %d", i, pm.Remote)
		}
	}

	// Validate transport
	if err := validateTransport(&cfg.Transport); err != nil {
		return err
	}

	// Validate resilience
	if err := validateResilience(&cfg.Resilience); err != nil {
		return err
	}

	return nil
}

// validateServerConfig validates server-specific configuration
func validateServerConfig(cfg *Config) error {
	// Validate listen address
	if cfg.Listen.Address == "" {
		return errors.New("listen address is required for server mode")
	}

	// Validate listen address format
	if _, _, err := net.SplitHostPort(cfg.Listen.Address); err != nil {
		return fmt.Errorf("invalid listen address format: %w", err)
	}

	// Validate path
	if cfg.Listen.Path == "" {
		cfg.Listen.Path = "/tunnel"
	}

	// Validate TLS if enabled
	if cfg.TLS.Enabled {
		if cfg.TLS.CertFile == "" {
			return errors.New("TLS cert_file is required when TLS is enabled")
		}
		if cfg.TLS.KeyFile == "" {
			return errors.New("TLS key_file is required when TLS is enabled")
		}
	}

	// Validate allowed ports if specified
	for _, port := range cfg.Security.AllowedPorts {
		if port <= 0 || port > 65535 {
			return fmt.Errorf("invalid allowed port: %d", port)
		}
	}

	// Validate transport
	if err := validateTransport(&cfg.Transport); err != nil {
		return err
	}

	return nil
}

// validateTransport validates transport configuration
func validateTransport(cfg *TransportConfig) error {
	// Validate protocol
	switch cfg.Protocol {
	case "http", "websocket":
		// Valid protocols
	default:
		return fmt.Errorf("invalid transport protocol: %s, must be 'http' or 'websocket'", cfg.Protocol)
	}

	// Validate packet size
	if cfg.PacketSize < 512 || cfg.PacketSize > 1048576 {
		return fmt.Errorf("packet_size must be between 512 and 1048576 bytes, got %d", cfg.PacketSize)
	}

	// Validate buffer size
	if cfg.BufferSize < 1024 || cfg.BufferSize > 16777216 {
		return fmt.Errorf("buffer_size must be between 1024 and 16777216 bytes, got %d", cfg.BufferSize)
	}

	return nil
}

// validateResilience validates resilience configuration
func validateResilience(cfg *ResilienceConfig) error {
	if cfg.HealthCheckInterval <= 0 {
		return errors.New("health_check_interval must be positive")
	}

	if cfg.ReconnectMinDelay <= 0 {
		return errors.New("reconnect_min_delay must be positive")
	}

	if cfg.ReconnectMaxDelay <= 0 {
		return errors.New("reconnect_max_delay must be positive")
	}

	if cfg.ReconnectMinDelay > cfg.ReconnectMaxDelay {
		return errors.New("reconnect_min_delay must be less than or equal to reconnect_max_delay")
	}

	if cfg.ConnectionTimeout <= 0 {
		return errors.New("connection_timeout must be positive")
	}

	if cfg.RequestTimeout <= 0 {
		return errors.New("request_timeout must be positive")
	}

	return nil
}
