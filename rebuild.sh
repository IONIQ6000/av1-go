#!/bin/bash
set -e

# Rebuild script for AV1 Daemon
# Rebuilds the Go binaries and optionally reinstalls them

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_PREFIX="${INSTALL_PREFIX:-/usr/local}"
BIN_DIR="${INSTALL_PREFIX}/bin"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Check if Go is available
if ! command -v go &> /dev/null; then
    log_warn "Go is not in PATH. Trying /usr/local/go/bin/go..."
    if [ -f "/usr/local/go/bin/go" ]; then
        export PATH=$PATH:/usr/local/go/bin
    else
        echo "Error: Go is not installed. Please install Go 1.25+ first."
        exit 1
    fi
fi

log_info "Building AV1 daemon binaries..."

cd "${SCRIPT_DIR}"

# Ensure dependencies are up to date
log_info "Updating dependencies..."
go mod download
go mod tidy

# Build av1d daemon
log_info "Building av1d..."
go build -o "${SCRIPT_DIR}/av1d" ./cmd/av1d
log_info "✓ av1d built successfully"

# Build av1top TUI
log_info "Building av1top..."
go build -o "${SCRIPT_DIR}/av1top" ./cmd/av1top
log_info "✓ av1top built successfully"

# Check if user wants to install
if [ "$1" == "--install" ] || [ "$1" == "-i" ]; then
    if [ "$EUID" -ne 0 ]; then
        log_warn "Installation requires root. Run with sudo:"
        echo "  sudo $0 --install"
        exit 1
    fi
    
    log_info "Installing binaries to ${BIN_DIR}..."
    cp "${SCRIPT_DIR}/av1d" "${BIN_DIR}/av1d"
    cp "${SCRIPT_DIR}/av1top" "${BIN_DIR}/av1top"
    chmod +x "${BIN_DIR}/av1d"
    chmod +x "${BIN_DIR}/av1top"
    
    log_info "Binaries installed successfully!"
    log_info "Restart the service with: sudo systemctl restart av1d"
else
    log_info "Build complete! Binaries are in: ${SCRIPT_DIR}"
    echo ""
    echo "To install system-wide, run:"
    echo "  sudo $0 --install"
    echo ""
    echo "Or manually copy:"
    echo "  sudo cp av1d av1top ${BIN_DIR}/"
    echo "  sudo systemctl restart av1d"
fi

