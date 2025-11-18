#!/bin/bash
set -e

# Fix GPU device access for the av1d service user

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

APP_USER="av1d"

log_info "Fixing GPU device access for user: ${APP_USER}"

# Check if user exists
if ! id "$APP_USER" &>/dev/null; then
    log_error "User ${APP_USER} does not exist!"
    exit 1
fi

# Add user to video and render groups for GPU access
log_info "Adding ${APP_USER} to video and render groups..."
usermod -a -G video,render "$APP_USER" || log_warn "Failed to add to groups (may already be member)"

# Check GPU devices
log_info "Checking GPU devices..."
if [ -d "/dev/dri" ]; then
    log_info "GPU devices found in /dev/dri:"
    ls -la /dev/dri/
    
    # Check permissions
    for device in /dev/dri/*; do
        if [ -c "$device" ]; then
            log_info "  Device: $device"
            stat -c "    Permissions: %a Owner: %U:%G" "$device"
        fi
    done
else
    log_warn "/dev/dri directory not found - GPU may not be available"
fi

# Verify group membership
log_info "Verifying group membership..."
groups "$APP_USER" | grep -q video && log_info "✓ User is in video group" || log_warn "User NOT in video group"
groups "$APP_USER" | grep -q render && log_info "✓ User is in render group" || log_warn "User NOT in render group"

# Test VA-API access
log_info "Testing VA-API access..."
if command -v vainfo &> /dev/null; then
    log_info "Running vainfo as ${APP_USER}..."
    sudo -u "$APP_USER" vainfo 2>&1 | head -20 || log_warn "vainfo failed - GPU may not be accessible"
else
    log_warn "vainfo not installed. Install with: sudo apt-get install vainfo"
fi

log_info "Done!"
echo ""
echo "Next steps:"
echo "1. Reload systemd: sudo systemctl daemon-reload"
echo "2. Restart service: sudo systemctl restart av1d"
echo "3. Check logs: sudo journalctl -u av1d -f"

