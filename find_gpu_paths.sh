#!/bin/bash
# Find all available GPU-related paths

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_info "=== Finding GPU Paths ==="
echo ""

# Find Intel GPU card
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
    echo "No Intel GPU found"
    exit 1
fi

echo ""
log_info "Exploring $CARD structure:"
find "$CARD" -type f -name "*freq*" -o -name "*busy*" -o -name "*util*" 2>/dev/null | head -20

echo ""
log_info "Checking device path:"
DEVICE="$CARD/device"
if [ -d "$DEVICE" ]; then
    log_info "Device directory exists"
    ls -la "$DEVICE" | head -10
fi

echo ""
log_info "Checking for gt directories:"
find "$DEVICE" -type d -name "gt*" 2>/dev/null

echo ""
log_info "Checking for engine directories:"
find "$DEVICE" -type d -name "engine*" 2>/dev/null

echo ""
log_info "Checking /sys/kernel/debug/dri (if accessible):"
if [ -d "/sys/kernel/debug/dri" ]; then
    find "/sys/kernel/debug/dri" -type f -name "*busy*" -o -name "*util*" 2>/dev/null | head -10
else
    echo "  Not accessible (may need debugfs mounted)"
fi

