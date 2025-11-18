#!/bin/bash
set -e

# AV1 Daemon Installation Script for Debian/Ubuntu
# This script builds and installs the AV1 transcoding daemon

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_PREFIX="${INSTALL_PREFIX:-/usr/local}"
BIN_DIR="${INSTALL_PREFIX}/bin"
SYSTEMD_DIR="/etc/systemd/system"
APP_USER="${APP_USER:-av1d}"
APP_GROUP="${APP_GROUP:-av1d}"
DATA_DIR="/var/lib/av1qsvd"
CONFIG_DIR="/etc/av1qsvd"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    log_error "Please run as root (use sudo)"
    exit 1
fi

log_info "Starting AV1 Daemon installation..."

# Check for Go
if ! command -v go &> /dev/null; then
    log_warn "Go is not installed. Installing Go 1.25..."
    
    # Install Go
    GO_VERSION="1.25"
    GO_ARCH="amd64"
    GO_TAR="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
    GO_URL="https://go.dev/dl/${GO_TAR}"
    
    cd /tmp
    wget -q "${GO_URL}"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "${GO_TAR}"
    rm "${GO_TAR}"
    
    # Add Go to PATH
    if ! grep -q "/usr/local/go/bin" /etc/profile; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
    fi
    export PATH=$PATH:/usr/local/go/bin
    
    log_info "Go ${GO_VERSION} installed"
else
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    log_info "Go ${GO_VERSION} found"
fi

# Install build dependencies
log_info "Installing build dependencies..."
apt-get update -qq
apt-get install -y -qq \
    build-essential \
    git \
    wget \
    tar \
    xz-utils \
    || log_warn "Some packages may have failed to install"

# Install Intel GPU/VA-API dependencies for QSV
log_info "Installing Intel GPU/VA-API dependencies..."
apt-get install -y -qq \
    libva-drm2 \
    libva2 \
    intel-media-va-driver-non-free \
    libdrm-intel1 \
    || log_warn "Some GPU packages may have failed to install"

log_info "GPU dependencies installed"

# Create application user and group
if ! id "$APP_USER" &>/dev/null; then
    log_info "Creating user ${APP_USER}..."
    useradd --system --no-create-home --shell /bin/false "${APP_USER}"
    APP_GROUP="${APP_USER}"
else
    log_info "User ${APP_USER} already exists"
    APP_GROUP=$(id -gn "${APP_USER}")
fi

# Create directories
log_info "Creating directories..."
mkdir -p "${BIN_DIR}"
mkdir -p "${DATA_DIR}/ffmpeg"
mkdir -p "${DATA_DIR}/jobs"
mkdir -p "${CONFIG_DIR}"
chown -R "${APP_USER}:${APP_GROUP}" "${DATA_DIR}"
chmod 755 "${DATA_DIR}"
chmod 755 "${DATA_DIR}/ffmpeg"
chmod 755 "${DATA_DIR}/jobs"

# Build the project
log_info "Building AV1 daemon..."
cd "${SCRIPT_DIR}"

# Ensure Go modules are downloaded
export PATH=$PATH:/usr/local/go/bin
go mod download
go mod tidy

# Build binaries
log_info "Compiling av1d daemon..."
go build -o "${BIN_DIR}/av1d" ./cmd/av1d
chmod +x "${BIN_DIR}/av1d"

log_info "Compiling av1top TUI..."
go build -o "${BIN_DIR}/av1top" ./cmd/av1top
chmod +x "${BIN_DIR}/av1top"

log_info "Binaries built successfully"

# Create systemd service file
log_info "Creating systemd service..."
cat > "${SYSTEMD_DIR}/av1d.service" <<EOF
[Unit]
Description=AV1 Transcoding Daemon
After=network.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}
WorkingDirectory=${DATA_DIR}
ExecStart=${BIN_DIR}/av1d
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

# GPU device access
SupplementaryGroups=video render media

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DATA_DIR} /dev/dri
ReadOnlyPaths=${CONFIG_DIR}

# Resource limits (adjust as needed)
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

# Create default configuration file
log_info "Creating default configuration..."
cat > "${CONFIG_DIR}/config.json" <<EOF
{
  "ffmpeg_url": "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n8.0-latest-linux64-gpl-8.0.tar.xz",
  "ffmpeg_install_dir": "${DATA_DIR}/ffmpeg",
  "library_roots": [],
  "min_bytes": 2147483648,
  "max_size_ratio": 0.90,
  "job_state_dir": "${DATA_DIR}/jobs",
  "scan_interval_sec": 60
}
EOF

chmod 644 "${CONFIG_DIR}/config.json"
chown root:root "${CONFIG_DIR}/config.json"

# Create log directory
mkdir -p /var/log/av1qsvd
chown "${APP_USER}:${APP_GROUP}" /var/log/av1qsvd

# Reload systemd
log_info "Reloading systemd..."
systemctl daemon-reload

log_info "Installation complete!"
echo ""
echo "Next steps:"
echo "1. Edit configuration: ${CONFIG_DIR}/config.json"
echo "   - Add your library_roots paths"
echo ""
echo "2. Start the daemon:"
echo "   sudo systemctl start av1d"
echo ""
echo "3. Enable auto-start on boot:"
echo "   sudo systemctl enable av1d"
echo ""
echo "4. Check status:"
echo "   sudo systemctl status av1d"
echo ""
echo "5. View logs:"
echo "   sudo journalctl -u av1d -f"
echo ""
echo "6. Use the TUI:"
echo "   ${BIN_DIR}/av1top"
echo ""

