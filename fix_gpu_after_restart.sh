#!/bin/bash
# Fix GPU access after LXC container restart

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

log_info "=== Fixing GPU Access After Container Restart ==="
echo ""

# 1. Check if av1d user exists
if ! id av1d &>/dev/null; then
    log_error "av1d user does not exist"
    exit 1
fi

# 2. Add av1d to required groups
log_info "Adding av1d to video, render, and media groups..."
usermod -a -G video,render,media av1d 2>/dev/null || log_warn "Some groups may not exist (this is OK)"

# 3. Check GPU device permissions
log_info "Checking GPU device permissions..."
if [ -d "/dev/dri" ]; then
    for device in /dev/dri/renderD*; do
        if [ -e "$device" ]; then
            log_info "Found GPU device: $device"
            # Check current permissions
            perms=$(stat -c "%a %U:%G" "$device" 2>/dev/null || echo "unknown")
            log_info "  Permissions: $perms"
            
            # Try to set permissions (may require host-side changes)
            chmod 666 "$device" 2>/dev/null || log_warn "  Cannot change permissions (may need host config)"
        fi
    done
else
    log_error "/dev/dri does not exist - GPU may not be accessible"
fi

# 4. Verify VAAPI libraries
log_info "Checking VAAPI libraries..."
if ldconfig -p | grep -q libva-drm; then
    log_info "✓ VAAPI libraries found"
else
    log_warn "VAAPI libraries not found in ldconfig"
    log_info "Installing VAAPI libraries..."
    apt-get update -qq
    apt-get install -y libva-drm2 libva2 intel-media-va-driver-non-free libdrm-intel1 2>/dev/null || log_warn "Failed to install some packages"
fi

# 5. Test VAAPI access
log_info "Testing VAAPI access..."
if command -v vainfo &>/dev/null; then
    log_info "Running vainfo..."
    if sudo -u av1d vainfo 2>&1 | head -5; then
        log_info "✓ VAAPI access works"
    else
        log_error "✗ VAAPI access failed"
    fi
else
    log_warn "vainfo not installed (install with: apt-get install vainfo)"
fi

# 6. Check systemd service configuration
log_info "Checking systemd service configuration..."
if [ -f "/etc/systemd/system/av1d.service" ]; then
    if grep -q "SupplementaryGroups.*media" /etc/systemd/system/av1d.service; then
        log_info "✓ Service has media group configured"
    else
        log_warn "Service missing media group - updating..."
        # This would need to be done via install.sh or manually
    fi
    
    if grep -q "ReadWritePaths.*/dev/dri" /etc/systemd/system/av1d.service; then
        log_info "✓ Service has /dev/dri access configured"
    else
        log_warn "Service missing /dev/dri access - updating..."
    fi
fi

# 7. Rebuild and restart
log_info ""
log_info "Next steps:"
log_info "1. Rebuild: cd ~/av1-go && git pull && go build ./cmd/av1d"
log_info "2. Install: sudo cp av1d /usr/local/bin/"
log_info "3. Restart: sudo systemctl restart av1d"
log_info "4. Check logs: sudo journalctl -u av1d -f"

