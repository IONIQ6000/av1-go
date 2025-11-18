#!/bin/bash
set -e

# Install Intel GPU/VA-API dependencies for QSV hardware acceleration

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
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

log_info "Installing Intel GPU/VA-API dependencies for QSV..."

# Update package list
apt-get update -qq

# Install VA-API libraries
log_info "Installing VA-API libraries..."
apt-get install -y -qq \
    libva-drm2 \
    libva2 \
    intel-media-va-driver-non-free \
    libdrm-intel1 \
    || log_warn "Some packages may have failed to install"

# Verify installation
log_info "Verifying installation..."
if ldconfig -p | grep -q libva-drm.so.2; then
    log_info "✓ libva-drm2 installed successfully"
else
    log_warn "libva-drm2 may not be properly installed"
fi

if ldconfig -p | grep -q libva.so.2; then
    log_info "✓ libva2 installed successfully"
else
    log_warn "libva2 may not be properly installed"
fi

log_info "Installation complete!"
echo ""
echo "Next steps:"
echo "1. Verify GPU is accessible:"
echo "   sudo vainfo"
echo ""
echo "2. Restart the daemon:"
echo "   sudo systemctl restart av1d"
echo ""
echo "3. Check logs:"
echo "   sudo journalctl -u av1d -f"
echo ""

