#!/bin/bash
# Check GPU permissions and group membership for av1d user

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

APP_USER="av1d"

log_info "=== GPU Permissions Check ==="
echo ""

# Check if user exists
if ! id "$APP_USER" &>/dev/null; then
    log_error "User ${APP_USER} does not exist!"
    exit 1
fi

# Check group membership
log_info "User: ${APP_USER}"
USER_GROUPS=$(groups "$APP_USER")
echo "Groups: $USER_GROUPS"
echo ""

if echo "$USER_GROUPS" | grep -q "video"; then
    log_info "✓ User IS in 'video' group"
else
    log_error "✗ User NOT in 'video' group"
    echo "  Fix: sudo usermod -a -G video ${APP_USER}"
fi

if echo "$USER_GROUPS" | grep -q "render"; then
    log_info "✓ User IS in 'render' group"
else
    log_error "✗ User NOT in 'render' group"
    echo "  Fix: sudo usermod -a -G render ${APP_USER}"
fi

echo ""

# Check GPU devices
log_info "GPU Devices:"
if [ -d "/dev/dri" ]; then
    for device in /dev/dri/*; do
        if [ -c "$device" ]; then
            PERMS=$(stat -c "%a" "$device")
            OWNER=$(stat -c "%U:%G" "$device")
            echo "  $device - Perms: $PERMS Owner: $OWNER"
        fi
    done
else
    log_error "/dev/dri directory not found!"
fi

echo ""

# Check if vainfo works
log_info "Testing VA-API as ${APP_USER}:"
if command -v vainfo &> /dev/null; then
    if sudo -u "$APP_USER" vainfo >/dev/null 2>&1; then
        log_info "✓ vainfo works for ${APP_USER}"
    else
        log_error "✗ vainfo FAILS for ${APP_USER}"
        echo ""
        echo "Output:"
        sudo -u "$APP_USER" vainfo 2>&1 | head -20
    fi
else
    log_warn "vainfo not installed (install with: sudo apt-get install vainfo)"
fi

echo ""
log_info "=== Summary ==="
echo "If checks failed:"
echo "  1. Run: sudo ./fix_gpu_access.sh"
echo "  2. Restart service: sudo systemctl restart av1d"
echo "  3. Check logs: sudo journalctl -u av1d -f"

