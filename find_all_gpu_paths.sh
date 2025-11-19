#!/bin/bash
# Comprehensive search for GPU metrics paths

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

CARD="/sys/class/drm/card1"
DEVICE="$CARD/device"

log_info "=== Comprehensive GPU Path Search ==="
echo ""

log_info "1. All files under $DEVICE:"
find "$DEVICE" -type f 2>/dev/null | head -30

echo ""
log_info "2. All directories under $DEVICE:"
find "$DEVICE" -type d 2>/dev/null | head -30

echo ""
log_info "3. Checking /sys/kernel/debug:"
if [ -d "/sys/kernel/debug" ]; then
    log_info "  /sys/kernel/debug exists"
    ls -la /sys/kernel/debug/ 2>/dev/null | head -10
    if [ -d "/sys/kernel/debug/dri" ]; then
        log_info "  /sys/kernel/debug/dri exists"
        ls -la /sys/kernel/debug/dri/ 2>/dev/null
    fi
else
    log_warn "  /sys/kernel/debug doesn't exist"
fi

echo ""
log_info "4. Checking /sys/devices for i915:"
find /sys/devices -name "*i915*" -type d 2>/dev/null | head -10

echo ""
log_info "5. Checking PCI device:"
PCI_PATH=$(readlink -f "$DEVICE" 2>/dev/null)
if [ -n "$PCI_PATH" ]; then
    log_info "  PCI path: $PCI_PATH"
    find "$PCI_PATH" -name "*freq*" -o -name "*busy*" -o -name "*util*" 2>/dev/null | head -20
fi

echo ""
log_info "6. Checking if intel_gpu_top works interactively:"
log_info "  (This will show output - press Ctrl+C after a moment)"
timeout 2 intel_gpu_top -l -s 1000 2>&1 | head -30

