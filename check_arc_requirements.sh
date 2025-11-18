#!/bin/bash
# Check Intel Arc GPU requirements on Debian

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

log_info "=== Intel Arc GPU Requirements Check ==="
echo ""

# Check kernel version
KERNEL_VERSION=$(uname -r)
KERNEL_MAJOR=$(echo $KERNEL_VERSION | cut -d. -f1)
KERNEL_MINOR=$(echo $KERNEL_VERSION | cut -d. -f2)

log_info "Kernel: $KERNEL_VERSION"
if [ "$KERNEL_MAJOR" -gt 6 ] || ([ "$KERNEL_MAJOR" -eq 6 ] && [ "$KERNEL_MINOR" -ge 2 ]); then
    log_info "✓ Kernel 6.2+ (Arc GPU fully supported)"
elif [ "$KERNEL_MAJOR" -eq 6 ] && [ "$KERNEL_MINOR" -ge 0 ]; then
    log_warn "⚠ Kernel 6.0-6.1 (Arc GPU partially supported, 6.2+ recommended)"
else
    log_error "✗ Kernel < 6.0 (Arc GPU NOT supported)"
    echo "  Upgrade kernel to 6.2+ for Intel Arc support"
fi
echo ""

# Check for Arc firmware
log_info "Checking Intel Arc firmware:"
FIRMWARE_DIR="/lib/firmware/i915"
if [ -d "$FIRMWARE_DIR" ]; then
    for fw in dg2_dmc_ver2_08.bin dg2_guc_70.bin dg2_huc_gsc.bin; do
        if [ -f "$FIRMWARE_DIR/$fw" ]; then
            log_info "✓ Found $fw"
        else
            log_warn "✗ Missing $fw"
        fi
    done
else
    log_error "✗ Firmware directory not found: $FIRMWARE_DIR"
    echo "  Install: sudo apt-get install firmware-misc-nonfree"
fi
echo ""

# Check VA-API driver
log_info "Checking VA-API driver:"
if vainfo 2>&1 | grep -q "iHD"; then
    log_info "✓ Intel iHD driver loaded (correct for Arc GPUs)"
    vainfo 2>&1 | grep -i "driver\|arc\|dg2"
elif vainfo 2>&1 | grep -q "i965"; then
    log_warn "⚠ Intel i965 driver loaded (old driver, Arc needs iHD)"
    echo "  Install: sudo apt-get install intel-media-va-driver"
else
    log_error "✗ No Intel VA-API driver found"
fi
echo ""

# Check for oneVPL
log_info "Checking for oneVPL:"
if ldconfig -p | grep -q libvpl; then
    log_info "✓ oneVPL runtime found"
    ldconfig -p | grep libvpl
else
    log_warn "✗ oneVPL runtime NOT found"
    echo "  Intel Arc GPUs use oneVPL instead of old Media SDK"
    echo "  Try: sudo apt-get install intel-onevpl intel-onevpl-utils"
fi
echo ""

# Check GPU recognition
log_info "GPU Information:"
if command -v lspci &> /dev/null; then
    lspci | grep -i vga
    lspci | grep -i display
fi
echo ""

log_info "=== Recommendations ==="
echo "For Intel Arc A310 on Debian:"
echo "  1. Upgrade kernel to 6.2+ if not already"
echo "  2. Install: sudo apt-get install firmware-misc-nonfree linux-firmware"
echo "  3. Install: sudo apt-get install intel-media-va-driver"
echo "  4. Install: sudo apt-get install intel-onevpl intel-onevpl-utils (if available)"
echo "  5. Reboot after driver/firmware changes"
echo ""
echo "After installing, test with:"
echo "  vainfo"
echo "  ffmpeg -init_hw_device vaapi=va:/dev/dri/renderD128 -init_hw_device qsv@va -hwaccel qsv -f lavfi -i testsrc -frames 1 -c:v av1_qsv -f null -"

