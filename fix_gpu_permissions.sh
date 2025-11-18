#!/bin/bash
set -e

# Fix GPU device permissions for av1d service

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

log_info "Fixing GPU device permissions for ${APP_USER}..."

# Ensure user exists
if ! id "$APP_USER" &>/dev/null; then
    log_error "User ${APP_USER} does not exist!"
    exit 1
fi

# Add user to video and render groups
log_info "Adding ${APP_USER} to video and render groups..."
usermod -a -G video,render "$APP_USER" 2>/dev/null || log_warn "User may already be in groups"

# Check current groups
log_info "User groups: $(groups $APP_USER)"

# Fix device permissions
log_info "Setting GPU device permissions..."
if [ -c "/dev/dri/renderD128" ]; then
    chmod 666 /dev/dri/renderD128
    log_info "✓ Set permissions on /dev/dri/renderD128"
else
    log_warn "/dev/dri/renderD128 not found"
fi

if [ -c "/dev/dri/card0" ]; then
    chmod 666 /dev/dri/card0
    log_info "✓ Set permissions on /dev/dri/card0"
fi

if [ -c "/dev/dri/card1" ]; then
    chmod 666 /dev/dri/card1
    log_info "✓ Set permissions on /dev/dri/card1"
fi

# Create udev rule for persistent permissions
log_info "Creating udev rule for persistent GPU access..."
UDEV_RULE="/etc/udev/rules.d/99-av1d-gpu.rules"
cat > "$UDEV_RULE" <<'EOF'
# Allow av1d user access to GPU devices
KERNEL=="renderD*", GROUP="render", MODE="0666"
KERNEL=="card*", GROUP="video", MODE="0666"
EOF

log_info "✓ Created udev rule: $UDEV_RULE"

# Reload udev rules
log_info "Reloading udev rules..."
udevadm control --reload-rules
udevadm trigger

# Verify access
log_info "Testing GPU access as ${APP_USER}..."
if sudo -u "$APP_USER" vainfo >/dev/null 2>&1; then
    log_info "✓ GPU access verified!"
    sudo -u "$APP_USER" vainfo 2>&1 | grep -i "AV1\|driver" | head -5
else
    log_warn "GPU access test failed - may need to restart service or reboot"
fi

log_info "Done!"
echo ""
echo "Next steps:"
echo "1. Reload systemd: sudo systemctl daemon-reload"
echo "2. Restart service: sudo systemctl restart av1d"
echo "3. Check logs: sudo journalctl -u av1d -f"
echo ""
echo "Note: If permissions don't persist, you may need to reboot"

