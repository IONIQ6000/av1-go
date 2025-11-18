#!/bin/bash
# Fix write access issues for av1d service

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

log_info "=== Fixing Write Access for av1d Service ==="
echo ""

# 1. Check ZFS dataset (if zfs is available)
if command -v zfs &>/dev/null; then
    log_info "Checking ZFS dataset properties..."
    DATASET=$(zfs list -o name,mountpoint | grep "/main-library-2" | awk '{print $1}' | head -1)
    if [ -n "$DATASET" ]; then
        READONLY=$(zfs get -H -o value readonly "$DATASET" 2>/dev/null)
        if [ "$READONLY" = "on" ]; then
            log_warn "ZFS dataset is readonly, fixing..."
            sudo zfs set readonly=off "$DATASET"
            log_info "✓ Set readonly=off on $DATASET"
        else
            log_info "✓ ZFS dataset is not readonly"
        fi
    fi
else
    log_warn "zfs command not available, skipping ZFS check"
fi
echo ""

# 2. Ensure ownership is correct
log_info "Verifying ownership..."
sudo chown -R av1d:av1d /main-library-2/Media/Movies 2>/dev/null
log_info "✓ Ownership set to av1d:av1d"
echo ""

# 3. Check AppArmor/SELinux
log_info "Checking security modules..."

# AppArmor
if [ -d /etc/apparmor.d ]; then
    if aa-status 2>/dev/null | grep -q av1d; then
        log_warn "AppArmor profile found for av1d"
        echo "  Check: sudo aa-status | grep av1d"
    else
        log_info "✓ No AppArmor restrictions found"
    fi
fi

# SELinux
if command -v getenforce &>/dev/null; then
    if [ "$(getenforce)" != "Disabled" ]; then
        log_warn "SELinux is enabled: $(getenforce)"
        echo "  Check: sudo getsebool -a | grep av1d"
    else
        log_info "✓ SELinux is disabled"
    fi
fi
echo ""

# 4. Restart service
log_info "Restarting av1d service..."
sudo systemctl restart av1d
sleep 2

if systemctl is-active --quiet av1d; then
    log_info "✓ Service restarted successfully"
else
    log_error "✗ Service failed to start"
    echo "  Check: sudo systemctl status av1d"
fi
echo ""

# 5. Test as the service user
log_info "Testing write access as av1d user (simulating service)..."
TEST_DIR="/main-library-2/Media/Movies"
TEST_FILE="$TEST_DIR/.av1d-service-test-$$"

# Run as av1d user with same environment as systemd service
if sudo -u av1d env HOME=/var/lib/av1d touch "$TEST_FILE" 2>/dev/null; then
    log_info "✓ av1d user can create files"
    sudo -u av1d rm -f "$TEST_FILE" 2>/dev/null
    
    # Test the actual temp file pattern
    TMP_FILE="$TEST_DIR/test.av1-tmp.mkv"
    if sudo -u av1d env HOME=/var/lib/av1d touch "$TMP_FILE" 2>/dev/null; then
        log_info "✓ av1d user can create .av1-tmp.mkv files"
        sudo -u av1d rm -f "$TMP_FILE" 2>/dev/null
    else
        log_error "✗ Cannot create .av1-tmp.mkv files"
        echo "  Error: $(sudo -u av1d env HOME=/var/lib/av1d touch "$TMP_FILE" 2>&1)"
    fi
else
    log_error "✗ Cannot create files"
    echo "  Error: $(sudo -u av1d env HOME=/var/lib/av1d touch "$TEST_FILE" 2>&1)"
fi
echo ""

log_info "=== Next Steps ==="
echo "1. Monitor logs: sudo journalctl -u av1d -f"
echo "2. If still failing, check:"
echo "   - AppArmor: sudo aa-status"
echo "   - SELinux: sudo getenforce"
echo "   - Process capabilities: sudo getpcaps \$(pgrep av1d)"
echo "3. Test ffmpeg directly:"
echo "   sudo -u av1d /path/to/ffmpeg -f lavfi -i testsrc=duration=1:size=320x240 -c:v av1_qsv test.av1-tmp.mkv"

