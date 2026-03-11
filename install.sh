#!/bin/bash
set -euo pipefail

REPO="IllicLanthresh/vertex"
INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="vertex"

red() { printf '\033[0;31m%s\033[0m\n' "$*"; }
green() { printf '\033[0;32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[0;33m%s\033[0m\n' "$*"; }

if [ "$(id -u)" -ne 0 ]; then
    red "This script must be run as root (use sudo)"
    exit 1
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [ "$OS" != "linux" ]; then
    red "Vertex only supports Linux. Detected: $OS"
    exit 1
fi

ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) red "Unsupported architecture: $ARCH"; exit 1 ;;
esac

if ! command -v curl &> /dev/null; then
    red "curl is required but not installed"
    exit 1
fi

green "Detecting latest Vertex release..."
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$LATEST" ]; then
    red "Failed to determine latest version"
    exit 1
fi

BINARY="vertex-linux-${ARCH}"
BASE_URL="https://github.com/${REPO}/releases/download/${LATEST}"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

green "Downloading Vertex ${LATEST} (linux/${ARCH})..."
curl -fsSL "${BASE_URL}/${BINARY}" -o "${TMPDIR}/${BINARY}"
curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMPDIR}/checksums.txt"

green "Verifying checksum..."
EXPECTED=$(grep "${BINARY}" "${TMPDIR}/checksums.txt" | awk '{print $1}')
ACTUAL=$(sha256sum "${TMPDIR}/${BINARY}" | awk '{print $1}')
if [ "$EXPECTED" != "$ACTUAL" ]; then
    red "Checksum verification failed!"
    red "  Expected: ${EXPECTED}"
    red "  Got:      ${ACTUAL}"
    exit 1
fi
green "Checksum OK"

mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/vertex"
chmod +x "${INSTALL_DIR}/vertex"

# Set up systemd service if systemctl is available
if command -v systemctl &> /dev/null && [ -d /etc/systemd/system ]; then
    cat > "/etc/systemd/system/${SERVICE_NAME}.service" << 'EOF'
[Unit]
Description=Vertex Traffic Generator
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/vertex --headless
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload

    green "Vertex ${LATEST} installed successfully!"
    echo ""
    echo "Run it:"
    echo "  sudo vertex"
    echo ""
    echo "Optional — run as a background service instead:"
    echo "  sudo systemctl start vertex"
    echo "  sudo systemctl enable vertex   # auto-start on boot"
    echo "  sudo journalctl -u vertex -f   # view logs"
else
    green "Vertex ${LATEST} installed successfully!"
    echo ""
    echo "Run it:"
    echo "  sudo vertex"
    echo ""
    yellow "Note: systemd not detected — skipping service setup."
    yellow "To run in the background without systemd:"
    yellow "  sudo vertex --headless &"
fi
