#!/bin/bash
# Check what GPU metrics are actually accessible

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

log_info "=== GPU Access Check ==="
echo ""

# Find Intel GPU
CARD=""
for card in /sys/class/drm/card*; do
    if [ -d "$card" ]; then
        vendorPath="$card/device/vendor"
        if [ -f "$vendorPath" ]; then
            vendor=$(cat "$vendorPath" 2>/dev/null)
            if echo "$vendor" | grep -q "8086"; then
                CARD="$card"
                log_info "Found Intel GPU: $card"
                break
            fi
        fi
    fi
done

if [ -z "$CARD" ]; then
    log_error "No Intel GPU found"
    exit 1
fi

DEVICE="$CARD/device"

echo ""
log_info "Checking debugfs:"
if mount | grep -q debugfs; then
    log_info "✓ debugfs is mounted"
    if [ -d "/sys/kernel/debug/dri" ]; then
        log_info "  /sys/kernel/debug/dri exists"
        ls -la /sys/kernel/debug/dri/ 2>/dev/null | head -5
    else
        log_warn "  /sys/kernel/debug/dri not found"
    fi
else
    log_warn "✗ debugfs not mounted"
    log_info "  To mount: sudo mount -t debugfs none /sys/kernel/debug"
fi

echo ""
log_info "Checking all readable files under $DEVICE:"
find "$DEVICE" -type f -readable 2>/dev/null | grep -E "(freq|busy|util|engine)" | head -20

echo ""
log_info "Checking GT directories:"
if [ -d "$DEVICE/gt" ]; then
    for gt in "$DEVICE/gt"/gt*; do
        if [ -d "$gt" ]; then
            log_info "  Found: $gt"
            ls "$gt" 2>/dev/null | head -10
        fi
    done
else
    log_warn "  No gt directory found"
fi

echo ""
log_info "Testing intel_gpu_top with sudo:"
if sudo -n true 2>/dev/null; then
    log_info "  Running: sudo timeout 1 intel_gpu_top -l -s 500"
    sudo timeout 1 intel_gpu_top -l -s 500 2>&1 | head -20
else
    log_warn "  Cannot test sudo (password required)"
fi

echo ""
log_info "Checking /proc/driver/i915:"
if [ -d "/proc/driver/i915" ]; then
    log_info "  /proc/driver/i915 exists"
    ls -la /proc/driver/i915/ 2>/dev/null | head -10
else
    log_warn "  /proc/driver/i915 not found"
fi

