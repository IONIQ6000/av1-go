#!/bin/bash
# Fix GPU device access for av1d service

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

log_info "=== Fixing GPU Device Access ==="
echo ""

# Check GPU devices
log_info "Checking GPU devices:"
if [ -d /dev/dri ]; then
    ls -la /dev/dri/
    echo ""
    
    # Check renderD128 specifically
    if [ -c /dev/dri/renderD128 ]; then
        DEV_OWNER=$(stat -c '%U:%G' /dev/dri/renderD128 2>/dev/null || stat -f '%Su:%Sg' /dev/dri/renderD128 2>/dev/null)
        DEV_PERMS=$(stat -c '%a' /dev/dri/renderD128 2>/dev/null || stat -f '%OLp' /dev/dri/renderD128 2>/dev/null)
        DEV_GID=$(stat -c '%g' /dev/dri/renderD128 2>/dev/null || stat -f '%g' /dev/dri/renderD128 2>/dev/null)
        
        log_info "renderD128:"
        echo "  Owner: $DEV_OWNER"
        echo "  GID: $DEV_GID"
        echo "  Permissions: $DEV_PERMS"
        echo ""
        
        # Check if render group can access it
        RENDER_GID=$(getent group render | cut -d: -f3)
        if [ "$DEV_GID" = "$RENDER_GID" ]; then
            log_info "✓ Device is in 'render' group (GID $RENDER_GID)"
        else
            log_warn "⚠ Device GID ($DEV_GID) doesn't match render group"
        fi
        
        # Test access as av1d
        log_info "Testing device access as av1d user:"
        if sudo -u av1d test -r /dev/dri/renderD128 && sudo -u av1d test -w /dev/dri/renderD128; then
            log_info "✓ av1d can read and write device"
        else
            log_error "✗ av1d CANNOT access device"
            echo "  Fix: sudo chmod 666 /dev/dri/renderD128"
            echo "  Or: sudo chgrp render /dev/dri/renderD128 && sudo chmod g+rw /dev/dri/renderD128"
        fi
    else
        log_error "✗ /dev/dri/renderD128 not found"
    fi
else
    log_error "✗ /dev/dri directory not found"
fi
echo ""

# Check service file for device access
log_info "Checking systemd service file:"
SERVICE_FILE="/etc/systemd/system/av1d.service"
if [ -f "$SERVICE_FILE" ]; then
    if grep -q "ReadWritePaths.*dri" "$SERVICE_FILE"; then
        log_info "✓ Service file has ReadWritePaths for /dev/dri"
    else
        log_warn "⚠ Service file does NOT have ReadWritePaths for /dev/dri"
        echo ""
        log_info "Current ReadWritePaths:"
        grep "ReadWritePaths" "$SERVICE_FILE" || echo "  (not found)"
        echo ""
        log_info "To fix, add to service file:"
        echo "  ReadWritePaths=/dev/dri"
        echo ""
        log_info "Or update existing ReadWritePaths line to include /dev/dri"
    fi
    
    # Check SupplementaryGroups
    if grep -q "SupplementaryGroups.*render" "$SERVICE_FILE"; then
        log_info "✓ Service has 'render' in SupplementaryGroups"
    else
        log_warn "⚠ Service does NOT have 'render' in SupplementaryGroups"
    fi
else
    log_error "Service file not found: $SERVICE_FILE"
fi
echo ""

# Test vainfo as av1d
log_info "Testing VAAPI access:"
if command -v vainfo &>/dev/null; then
    log_info "Running: sudo -u av1d vainfo --display drm --device /dev/dri/renderD128"
    sudo -u av1d vainfo --display drm --device /dev/dri/renderD128 2>&1 | head -20
    if [ ${PIPESTATUS[0]} -eq 0 ]; then
        log_info "✓ vainfo succeeded - VAAPI access works!"
    else
        log_error "✗ vainfo failed - VAAPI access broken"
    fi
else
    log_warn "vainfo not found, skipping VAAPI test"
fi
echo ""

log_info "=== Fix Steps ==="
echo "1. Ensure device permissions:"
echo "   sudo chmod 666 /dev/dri/renderD128"
echo "   OR"
echo "   sudo chgrp render /dev/dri/renderD128"
echo "   sudo chmod g+rw /dev/dri/renderD128"
echo ""
echo "2. Update service file to allow device access:"
echo "   sudo nano /etc/systemd/system/av1d.service"
echo "   Add to ReadWritePaths: /dev/dri"
echo "   OR add new line: ReadWritePaths=/dev/dri"
echo ""
echo "3. Reload and restart:"
echo "   sudo systemctl daemon-reload"
echo "   sudo systemctl restart av1d"
echo ""
echo "4. Verify:"
echo "   sudo -u av1d vainfo --display drm --device /dev/dri/renderD128"

