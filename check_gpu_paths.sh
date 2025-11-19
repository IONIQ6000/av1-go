#!/bin/bash
# Diagnostic script to check what GPU paths are available for reading utilization

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

log_info "=== GPU Path Diagnostic ==="
echo ""

# Find Intel GPU card
log_info "Looking for Intel GPU cards:"
CARD_FOUND=""
for card in /sys/class/drm/card*; do
    if [ -d "$card" ]; then
        vendorPath="$card/device/vendor"
        if [ -f "$vendorPath" ]; then
            vendor=$(cat "$vendorPath" 2>/dev/null)
            if echo "$vendor" | grep -q "8086"; then
                CARD_FOUND="$card"
                log_info "✓ Found Intel GPU: $card"
                echo "  Vendor: $vendor"
                break
            fi
        fi
    fi
done

if [ -z "$CARD_FOUND" ]; then
    log_error "✗ No Intel GPU found"
    exit 1
fi

DEVICE_PATH="$CARD_FOUND/device"
GT_PATH="$DEVICE_PATH/gt/gt0"

echo ""
log_info "Checking engine busy files:"
ENGINES_PATH="$GT_PATH/engines"
if [ -d "$ENGINES_PATH" ]; then
    log_info "✓ Engines directory exists: $ENGINES_PATH"
    for engine in "$ENGINES_PATH"/*; do
        if [ -d "$engine" ]; then
            engineName=$(basename "$engine")
            busyFile="$engine/busy"
            if [ -f "$busyFile" ]; then
                busy=$(cat "$busyFile" 2>/dev/null)
                log_info "  Engine $engineName: busy=$busy"
            else
                log_warn "  Engine $engineName: no busy file"
            fi
        fi
    done
else
    log_error "✗ Engines directory not found: $ENGINES_PATH"
fi

echo ""
log_info "Checking frequency files:"
FREQ_PATHS=(
    "$GT_PATH/rps_act_freq_mhz"
    "$GT_PATH/rps_max_freq_mhz"
    "$GT_PATH/intel_gpu_freq"
    "$DEVICE_PATH/gt_act_freq_mhz"
    "$DEVICE_PATH/gt_max_freq_mhz"
)

for path in "${FREQ_PATHS[@]}"; do
    if [ -f "$path" ]; then
        value=$(cat "$path" 2>/dev/null)
        log_info "✓ $(basename $path): $value"
    else
        log_warn "✗ $(basename $path): not found"
    fi
done

echo ""
log_info "Testing intel_gpu_top:"
if command -v intel_gpu_top &>/dev/null; then
    log_info "✓ intel_gpu_top is available"
    echo ""
    log_info "Sample output (one-shot):"
    intel_gpu_top -l -n 1 2>&1 | head -20
else
    log_warn "✗ intel_gpu_top not found"
fi

echo ""
log_info "=== Summary ==="
echo "If engine busy files exist, GPU usage should work."
echo "If not, check permissions or kernel version."

