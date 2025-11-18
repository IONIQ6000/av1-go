#!/bin/bash
set -e

# Fix GPU library dependencies for Intel QSV

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

if [ "$EUID" -ne 0 ]; then
    log_error "Please run as root (use sudo)"
    exit 1
fi

log_info "Checking for missing VA-API libraries..."

# Check if libraries exist
MISSING_LIBS=0

if ! ldconfig -p | grep -q libva-drm.so.2; then
    log_warn "libva-drm.so.2 not found"
    MISSING_LIBS=1
else
    log_info "✓ libva-drm.so.2 found"
fi

if ! ldconfig -p | grep -q libva.so.2; then
    log_warn "libva.so.2 not found"
    MISSING_LIBS=1
else
    log_info "✓ libva.so.2 found"
fi

if [ $MISSING_LIBS -eq 0 ]; then
    log_info "All libraries appear to be installed!"
    echo ""
    echo "Checking library locations:"
    ldconfig -p | grep -E "libva|libdrm"
    exit 0
fi

log_info "Installing required packages..."

# Update package list
apt-get update -qq

# Try different package name variations
log_info "Attempting to install VA-API packages..."

# Try standard package names first
if apt-get install -y -qq libva-drm2 libva2 2>/dev/null; then
    log_info "✓ Installed libva-drm2 and libva2"
else
    log_warn "Standard packages not found, trying alternatives..."
    
    # Try alternative package names
    apt-get install -y -qq libva-drm2.2 libva2.2 2>/dev/null || \
    apt-get install -y -qq libva-drm2.1 libva2.1 2>/dev/null || \
    apt-get install -y -qq libva-drm libva 2>/dev/null || {
        log_error "Could not install VA-API packages automatically"
        echo ""
        echo "Please install manually. Try one of these:"
        echo "  sudo apt-get install libva-drm2 libva2"
        echo "  sudo apt-get install libva-drm libva"
        echo ""
        echo "Then run: sudo ldconfig"
        exit 1
    }
fi

# Install Intel media driver
log_info "Installing Intel media driver..."
if apt-get install -y -qq intel-media-va-driver-non-free 2>/dev/null; then
    log_info "✓ Installed intel-media-va-driver-non-free"
else
    log_warn "intel-media-va-driver-non-free not available, trying alternatives..."
    apt-get install -y -qq intel-media-va-driver 2>/dev/null || \
    apt-get install -y -qq i965-va-driver 2>/dev/null || \
    log_warn "Intel media driver package not found - may need to enable non-free repos"
fi

# Install DRM Intel driver
log_info "Installing DRM Intel driver..."
apt-get install -y -qq libdrm-intel1 2>/dev/null || \
apt-get install -y -qq libdrm-intel1.1 2>/dev/null || \
log_warn "libdrm-intel1 not found"

# Update library cache
log_info "Updating library cache..."
ldconfig

# Verify installation
log_info "Verifying installation..."
if ldconfig -p | grep -q libva-drm.so.2; then
    log_info "✓ libva-drm.so.2 is now available"
else
    log_error "libva-drm.so.2 still not found after installation"
    echo ""
    echo "Try manually:"
    echo "  sudo apt-get update"
    echo "  sudo apt-get install libva-drm2 libva2"
    echo "  sudo ldconfig"
    exit 1
fi

log_info "Installation complete!"
echo ""
echo "Testing VA-API access:"
if command -v vainfo &> /dev/null; then
    echo "Running vainfo..."
    vainfo 2>&1 | head -20 || log_warn "vainfo failed - GPU may not be accessible"
else
    log_warn "vainfo not installed. Install with: sudo apt-get install vainfo"
fi

echo ""
echo "Restart the daemon:"
echo "  sudo systemctl restart av1d"

