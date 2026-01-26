// Package config provides configuration loading and management using Viper
package config

import (
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete configuration
type Config struct {
	Mode       string           `mapstructure:"mode"`
	Server     ServerConfig     `mapstructure:"server"`
	Listen     ListenConfig     `mapstructure:"listen"`
	Ports      []PortMapping    `mapstructure:"ports"`
	Transport  TransportConfig  `mapstructure:"transport"`
	TLS        TLSConfig        `mapstructure:"tls"`
	Resilience ResilienceConfig `mapstructure:"resilience"`
	Security   SecurityConfig   `mapstructure:"security"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

// ServerConfig holds server connection settings for client mode
type ServerConfig struct {
	Address   string `mapstructure:"address"`
	ResolveIP string `mapstructure:"resolve_ip"`
}

// ListenConfig holds listen settings for server mode
type ListenConfig struct {
	Address string `mapstructure:"address"`
	Path    string `mapstructure:"path"`
}

// PortMapping defines a local to remote port mapping
type PortMapping struct {
	Local  int `mapstructure:"local"`
	Remote int `mapstructure:"remote"`
}

// TransportConfig holds transport layer settings
type TransportConfig struct {
	Protocol   string `mapstructure:"protocol"`
	Multiplex  bool   `mapstructure:"multiplex"`
	PacketSize int    `mapstructure:"packet_size"`
	Compress   bool   `mapstructure:"compress"`
	BufferSize int    `mapstructure:"buffer_size"`
}

// TLSConfig holds TLS settings
type TLSConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
	CAFile             string `mapstructure:"ca_file"`
	CertFile           string `mapstructure:"cert_file"`
	KeyFile            string `mapstructure:"key_file"`
}

// ResilienceConfig holds resilience settings
type ResilienceConfig struct {
	HealthCheckInterval time.Duration `mapstructure:"health_check_interval"`
	ReconnectMinDelay   time.Duration `mapstructure:"reconnect_min_delay"`
	ReconnectMaxDelay   time.Duration `mapstructure:"reconnect_max_delay"`
	ConnectionTimeout   time.Duration `mapstructure:"connection_timeout"`
	RequestTimeout      time.Duration `mapstructure:"request_timeout"`
}

// SecurityConfig holds security settings for server mode
type SecurityConfig struct {
	AuthToken    string `mapstructure:"auth_token"`
	AllowedPorts []int  `mapstructure:"allowed_ports"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	File   string `mapstructure:"file"`
}

// Load reads configuration from the specified file
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Set config file
	v.SetConfigFile(configPath)

	// Read config
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// setDefaults sets default values for configuration
func setDefaults(v *viper.Viper) {
	// Transport defaults
	v.SetDefault("transport.protocol", "http")
	v.SetDefault("transport.multiplex", true)
	v.SetDefault("transport.packet_size", 4096)
	v.SetDefault("transport.compress", true)
	v.SetDefault("transport.buffer_size", 65536)

	// Resilience defaults
	v.SetDefault("resilience.health_check_interval", "5s")
	v.SetDefault("resilience.reconnect_min_delay", "1s")
	v.SetDefault("resilience.reconnect_max_delay", "30s")
	v.SetDefault("resilience.connection_timeout", "10s")
	v.SetDefault("resilience.request_timeout", "30s")

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "text")

	// Listen defaults
	v.SetDefault("listen.path", "/tunnel")
}

// DefaultClientConfig returns a default client configuration
func DefaultClientConfig() *Config {
	return &Config{
		Mode: "client",
		Server: ServerConfig{
			Address:   "tunnel.example.com:443",
			ResolveIP: "",
		},
		Ports: []PortMapping{
			{Local: 3306, Remote: 3306},
			{Local: 6379, Remote: 6379},
			{Local: 8080, Remote: 8080},
		},
		Transport: TransportConfig{
			Protocol:   "http",
			Multiplex:  true,
			PacketSize: 4096,
			Compress:   true,
			BufferSize: 65536,
		},
		TLS: TLSConfig{
			Enabled:            true,
			InsecureSkipVerify: false,
		},
		Resilience: ResilienceConfig{
			HealthCheckInterval: 5 * time.Second,
			ReconnectMinDelay:   1 * time.Second,
			ReconnectMaxDelay:   30 * time.Second,
			ConnectionTimeout:   10 * time.Second,
			RequestTimeout:      30 * time.Second,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// DefaultServerConfig returns a default server configuration
func DefaultServerConfig() *Config {
	return &Config{
		Mode: "server",
		Listen: ListenConfig{
			Address: "0.0.0.0:443",
			Path:    "/tunnel",
		},
		Transport: TransportConfig{
			Protocol:   "http",
			Multiplex:  true,
			PacketSize: 4096,
			Compress:   true,
			BufferSize: 65536,
		},
		TLS: TLSConfig{
			Enabled:  true,
			CertFile: "/etc/cloudtunnel/cert.pem",
			KeyFile:  "/etc/cloudtunnel/key.pem",
		},
		Security: SecurityConfig{
			AuthToken:    "your-secret-token",
			AllowedPorts: []int{3306, 6379, 8080},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			File:   "/var/log/cloudtunnel/server.log",
		},
	}
}
