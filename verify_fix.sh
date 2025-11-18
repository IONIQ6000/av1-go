#!/bin/bash
# Quick verification that the fix worked

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

log_info "=== Verifying Fix ==="
echo ""

# Check av1d groups
log_info "av1d user groups:"
groups av1d
echo ""

# Restart service
log_info "Restarting av1d service..."
sudo systemctl restart av1d
sleep 2

if systemctl is-active --quiet av1d; then
    log_info "✓ Service is running"
else
    log_error "✗ Service failed to start"
    sudo systemctl status av1d --no-pager -l
    exit 1
fi
echo ""

# Test write access one more time
log_info "Final write test:"
TEST_FILE="/main-library-2/Media/Movies/.final-test-$$"
if sudo -u av1d touch "$TEST_FILE" 2>/dev/null; then
    log_info "✓ Write test passed"
    sudo -u av1d rm -f "$TEST_FILE"
else
    log_error "✗ Write test failed"
    echo "  Error: $(sudo -u av1d touch "$TEST_FILE" 2>&1)"
fi
echo ""

log_info "=== Monitoring Logs ==="
echo "Watch for successful transcoding (no more 'Read-only file system' errors):"
echo "  sudo journalctl -u av1d -f"
echo ""
echo "You should see jobs processing successfully now!"

