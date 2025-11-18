#!/bin/bash
# Verify systemd service configuration is correct

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

SERVICE_FILE="/etc/systemd/system/av1d.service"

log_info "=== Verifying Service Configuration ==="
echo ""

# Check service file exists
if [ ! -f "$SERVICE_FILE" ]; then
    log_error "Service file not found: $SERVICE_FILE"
    exit 1
fi

log_info "Service file: $SERVICE_FILE"
echo ""

# Check ReadWritePaths
log_info "Checking ReadWritePaths:"
RWP_LINE=$(grep "^ReadWritePaths=" "$SERVICE_FILE")
if [ -n "$RWP_LINE" ]; then
    echo "  $RWP_LINE"
    if echo "$RWP_LINE" | grep -q "/main-library-2"; then
        log_info "✓ /main-library-2 is in ReadWritePaths"
    else
        log_error "✗ /main-library-2 is NOT in ReadWritePaths"
        echo ""
        log_info "Current ReadWritePaths:"
        echo "  $RWP_LINE"
        echo ""
        log_info "Fixing..."
        sed -i 's|^ReadWritePaths=.*|ReadWritePaths=/var/lib/av1qsvd /dev/dri /main-library-2|' "$SERVICE_FILE"
        log_info "✓ Updated ReadWritePaths"
        echo ""
        log_info "New ReadWritePaths:"
        grep "^ReadWritePaths=" "$SERVICE_FILE"
    fi
else
    log_error "✗ ReadWritePaths not found in service file"
    log_info "Adding ReadWritePaths..."
    sed -i "/^ProtectHome=/a ReadWritePaths=/var/lib/av1qsvd /dev/dri /main-library-2" "$SERVICE_FILE"
    log_info "✓ Added ReadWritePaths"
fi
echo ""

# Check SupplementaryGroups
log_info "Checking SupplementaryGroups:"
SG_LINE=$(grep "^SupplementaryGroups=" "$SERVICE_FILE")
if [ -n "$SG_LINE" ]; then
    echo "  $SG_LINE"
    if echo "$SG_LINE" | grep -q "media"; then
        log_info "✓ media group is in SupplementaryGroups"
    else
        log_warn "⚠ media group NOT in SupplementaryGroups"
    fi
else
    log_warn "⚠ SupplementaryGroups not found"
fi
echo ""

# Check User
log_info "Checking User:"
USER_LINE=$(grep "^User=" "$SERVICE_FILE")
if [ -n "$USER_LINE" ]; then
    echo "  $USER_LINE"
    if echo "$USER_LINE" | grep -q "av1d"; then
        log_info "✓ Service runs as av1d user"
    else
        log_error "✗ Service does NOT run as av1d user"
    fi
else
    log_error "✗ User not specified"
fi
echo ""

# Check if systemd needs reload
log_info "Checking if systemd needs reload:"
if systemctl show av1d.service | grep -q "ReadWritePaths.*main-library"; then
    log_info "✓ Systemd has correct ReadWritePaths"
else
    log_warn "⚠ Systemd does NOT have /main-library-2 in ReadWritePaths"
    log_info "  Run: sudo systemctl daemon-reload"
fi
echo ""

log_info "=== Full Service File (relevant sections) ==="
grep -E "^User=|^Group=|^SupplementaryGroups=|^ReadWritePaths=|^ProtectSystem=" "$SERVICE_FILE"
echo ""

log_info "=== Next Steps ==="
echo "1. If ReadWritePaths was updated, reload systemd:"
echo "   sudo systemctl daemon-reload"
echo ""
echo "2. Restart service:"
echo "   sudo systemctl restart av1d"
echo ""
echo "3. Verify systemd has the paths:"
echo "   sudo systemctl show av1d.service | grep ReadWritePaths"
echo ""
echo "4. Check process can write:"
echo "   sudo -u av1d touch /main-library-2/Media/Movies/.test-write"
echo "   sudo -u av1d rm /main-library-2/Media/Movies/.test-write"

