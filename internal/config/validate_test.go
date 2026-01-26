package config

import (
	"testing"
	"time"
)

func TestValidateClientConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: true,
		},
		{
			name: "invalid mode",
			cfg: &Config{
				Mode: "invalid",
			},
			wantErr: true,
		},
		{
			name: "missing server address",
			cfg: &Config{
				Mode: "client",
			},
			wantErr: true,
		},
		{
			name: "invalid server address format",
			cfg: &Config{
				Mode: "client",
				Server: ServerConfig{
					Address: "invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid resolve_ip",
			cfg: &Config{
				Mode: "client",
				Server: ServerConfig{
					Address:   "localhost:8080",
					ResolveIP: "invalid-ip",
				},
			},
			wantErr: true,
		},
		{
			name: "no port mappings",
			cfg: &Config{
				Mode: "client",
				Server: ServerConfig{
					Address: "localhost:8080",
				},
				Ports: []PortMapping{},
			},
			wantErr: true,
		},
		{
			name: "invalid local port",
			cfg: &Config{
				Mode: "client",
				Server: ServerConfig{
					Address: "localhost:8080",
				},
				Ports: []PortMapping{
					{Local: 0, Remote: 8080},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid protocol",
			cfg: &Config{
				Mode: "client",
				Server: ServerConfig{
					Address: "localhost:8080",
				},
				Ports: []PortMapping{
					{Local: 8080, Remote: 8080},
				},
				Transport: TransportConfig{
					Protocol:   "invalid",
					PacketSize: 4096,
					BufferSize: 65536,
				},
			},
			wantErr: true,
		},
		{
			name: "valid client config",
			cfg: &Config{
				Mode: "client",
				Server: ServerConfig{
					Address: "localhost:8080",
				},
				Ports: []PortMapping{
					{Local: 8080, Remote: 8080},
				},
				Transport: TransportConfig{
					Protocol:   "http",
					PacketSize: 4096,
					BufferSize: 65536,
				},
				Resilience: ResilienceConfig{
					HealthCheckInterval: 5 * time.Second,
					ReconnectMinDelay:   1 * time.Second,
					ReconnectMaxDelay:   30 * time.Second,
					ConnectionTimeout:   10 * time.Second,
					RequestTimeout:      30 * time.Second,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateServerConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "missing listen address",
			cfg: &Config{
				Mode: "server",
			},
			wantErr: true,
		},
		{
			name: "TLS enabled without cert",
			cfg: &Config{
				Mode: "server",
				Listen: ListenConfig{
					Address: "0.0.0.0:443",
					Path:    "/tunnel",
				},
				TLS: TLSConfig{
					Enabled: true,
				},
				Transport: TransportConfig{
					Protocol:   "http",
					PacketSize: 4096,
					BufferSize: 65536,
				},
			},
			wantErr: true,
		},
		{
			name: "valid server config without TLS",
			cfg: &Config{
				Mode: "server",
				Listen: ListenConfig{
					Address: "0.0.0.0:8080",
					Path:    "/tunnel",
				},
				TLS: TLSConfig{
					Enabled: false,
				},
				Transport: TransportConfig{
					Protocol:   "http",
					PacketSize: 4096,
					BufferSize: 65536,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
