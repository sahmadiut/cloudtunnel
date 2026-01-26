# CloudTunnel

A production-grade, resilient TCP-over-HTTP tunnel written in Go. Designed for unstable networks with high packet loss and strict limitations.

## Overview

```
┌─────────────────────┐          HTTP/HTTPS           ┌─────────────────────┐
│      CLIENT         │ ◄──────────────────────────► │       SERVER        │
│                     │    (Compressed, Chunked)      │                     │
│  Local Ports:       │                               │  Forward to:        │
│  - 3306 ──────────────────────────────────────────► │  - 3306 (MySQL)     │
│  - 6379 ──────────────────────────────────────────► │  - 6379 (Redis)     │
│  - 8080 ──────────────────────────────────────────► │  - 8080 (App)       │
└─────────────────────┘                               └─────────────────────┘
```

## Features

| Category | Features |
|----------|----------|
| **Transport** | HTTP/HTTPS, WebSocket, HTTP/2 support |
| **Performance** | Configurable packet size, Zstd compression, Multiplexing |
| **Resilience** | Auto-reconnect, Health checks, Connection pooling |
| **Security** | TLS (optional), Token auth, Port whitelisting |
| **Networking** | `resolve_ip` for CDN bypass, Multi-port forwarding |
| **Operations** | Structured logging, Multiple log levels, File logging |
| **Deployment** | GitHub Actions CI/CD, Multi-platform builds, One-line installer |

## Installation

### One-liner installation

```bash
curl -sSL https://raw.githubusercontent.com/sahmadiut/cloudtunnel/main/scripts/install.sh | bash
```

### From source

```bash
go install github.com/sahmadiut/cloudtunnel/cmd/cloudtunnel@latest
```

### Build from source

```bash
git clone https://github.com/sahmadiut/cloudtunnel.git
cd cloudtunnel
go build -o cloudtunnel ./cmd/cloudtunnel
```

## Quick Start

### Generate configuration

```bash
# Generate client config
cloudtunnel init client > client.yaml

# Generate server config
cloudtunnel init server > server.yaml
```

### Run as server

```bash
cloudtunnel server -c server.yaml
```

### Run as client

```bash
cloudtunnel client -c client.yaml
```

### Access remote services locally

```bash
# Connect to remote MySQL
mysql -h 127.0.0.1 -P 3306 -u user -p

# Connect to remote Redis
redis-cli -p 6379

# Connect to remote app
curl http://127.0.0.1:8080
```

## Configuration

### Client Configuration

```yaml
mode: client

# Server connection
server:
  address: "tunnel.example.com:443"
  resolve_ip: "185.143.234.56"    # Optional: bypass DNS/CDN

# Port mappings (local -> remote)
ports:
  - local: 3306
    remote: 3306
  - local: 6379
    remote: 6379
  - local: 8080
    remote: 8080

# Transport settings
transport:
  protocol: http                   # http | websocket
  multiplex: true                  # Single connection for all ports
  packet_size: 4096                # Max bytes per HTTP request/response
  compress: true                   # Zstd compression
  buffer_size: 65536               # Read/write buffer

# TLS settings (optional)
tls:
  enabled: true
  insecure_skip_verify: false      # For self-signed certs
  ca_file: ""                      # Custom CA certificate

# Resilience settings
resilience:
  health_check_interval: 5s        # Ping interval
  reconnect_min_delay: 1s          # Initial retry delay
  reconnect_max_delay: 30s         # Max retry delay
  connection_timeout: 10s          # Dial timeout
  request_timeout: 30s             # HTTP request timeout

# Logging
logging:
  level: info                      # debug | info | warn | error
  format: text                     # text | json
  file: ""                         # Optional: log to file
```

### Server Configuration

```yaml
mode: server

# Listen address
listen:
  address: "0.0.0.0:443"
  path: "/tunnel"                  # HTTP endpoint path

# TLS settings (optional)
tls:
  enabled: true
  cert_file: "/etc/cloudtunnel/cert.pem"
  key_file: "/etc/cloudtunnel/key.pem"

# Transport settings
transport:
  protocol: http
  multiplex: true
  packet_size: 4096
  compress: true
  buffer_size: 65536

# Security
security:
  auth_token: "your-secret-token"  # Optional: bearer token auth
  allowed_ports: [3306, 6379, 8080] # Whitelist allowed ports

# Logging
logging:
  level: info
  format: json
  file: "/var/log/cloudtunnel/server.log"
```

## CLI Reference

```bash
# Run as client
cloudtunnel client -c /etc/cloudtunnel/client.yaml

# Run as server  
cloudtunnel server -c /etc/cloudtunnel/server.yaml

# Override config via flags
cloudtunnel client -c config.yaml --log-level debug --resolve-ip 1.2.3.4

# Generate example config
cloudtunnel init client > client.yaml
cloudtunnel init server > server.yaml

# Version info
cloudtunnel version
```

## Architecture

### Transport Layer

| Component | Description |
|-----------|-------------|
| **HTTP Transport** | Tunnels data via HTTP/1.1 chunked POST or HTTP/2 streams. Respects `packet_size` for both upload and download. |
| **WebSocket Transport** | Alternative transport using WebSocket for bidirectional streaming. |
| **Custom Dialer** | Implements `resolve_ip` - connects to specified IP while preserving SNI/Host header. |
| **Multiplexer** | Uses `yamux` to multiplex all port connections over a single HTTP connection. |

### Data Pipeline

```
TCP Data → Chunker (packet_size) → Compressor (zstd) → HTTP Transport → Network
                                                                          ↓
TCP Data ← Chunker (reassemble)  ← Decompressor      ← HTTP Transport ← Network
```

### Resilience System

| Feature | Implementation |
|---------|----------------|
| **Health Check** | Periodic ping/pong over the tunnel connection |
| **Auto Reconnect** | Exponential backoff (1s → 30s) using `cenkalti/backoff/v4` |
| **Connection Pool** | Maintains ready connections for quick failover |
| **Graceful Degradation** | Queues data during brief disconnections |

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Config management (YAML/TOML) |
| `github.com/rs/zerolog` | Structured logging |
| `github.com/hashicorp/yamux` | Connection multiplexing |
| `github.com/klauspost/compress/zstd` | Fast compression |
| `github.com/cenkalti/backoff/v4` | Exponential backoff |
| `github.com/gorilla/websocket` | WebSocket transport |

## License

MIT