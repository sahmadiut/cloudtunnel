#!/bin/bash
set -e

REPO="sahmadiut/cloudtunnel"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/cloudtunnel"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest version
VERSION=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
if [ -z "$VERSION" ]; then
    echo "Failed to fetch latest version"
    exit 1
fi

echo "Installing cloudtunnel ${VERSION} for ${OS}/${ARCH}..."

# Download and extract
FILENAME="cloudtunnel_${VERSION#v}_${OS}_${ARCH}.tar.gz"
curl -sL "https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}" | tar xz -C /tmp

# Install binary
sudo mv /tmp/cloudtunnel "$INSTALL_DIR/"
sudo chmod +x "$INSTALL_DIR/cloudtunnel"

# Create config directory
sudo mkdir -p "$CONFIG_DIR"

echo "✓ Installed cloudtunnel to $INSTALL_DIR/cloudtunnel"
echo ""
echo "Quick start:"
echo "  cloudtunnel init client > $CONFIG_DIR/client.yaml"
echo "  cloudtunnel client -c $CONFIG_DIR/client.yaml"
