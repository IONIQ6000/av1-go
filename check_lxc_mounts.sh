#!/bin/bash
# Check LXC container mount configuration for write access

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

log_info "=== LXC Container Mount Check ==="
echo ""

# Check if we're in an LXC container
if [ -f /proc/1/environ ] && grep -q container=lxc /proc/1/environ 2>/dev/null; then
    log_info "✓ Running inside LXC container"
    CONTAINER_NAME=$(hostname)
    log_info "Container name: $CONTAINER_NAME"
elif [ -d /sys/fs/cgroup ] && mount | grep -q cgroup; then
    log_info "Checking if running in container..."
    if [ -f /.dockerenv ] || [ -f /.containerenv ]; then
        log_info "Container detected (Docker/Podman)"
    else
        log_info "May be in LXC container (checking mounts)"
    fi
else
    log_warn "Not clearly in a container, but checking mounts anyway"
fi
echo ""

# Check mount options for /main-library-2
log_info "Checking mount options for /main-library-2:"
MOUNT_INFO=$(mount | grep "/main-library-2")
if [ -n "$MOUNT_INFO" ]; then
    echo "$MOUNT_INFO"
    
    # Extract mount options
    MOUNT_OPTS=$(echo "$MOUNT_INFO" | grep -oP '\([^)]+\)' | tr -d '()')
    echo ""
    log_info "Mount options: $MOUNT_OPTS"
    
    # Check for read-only indicators
    if echo "$MOUNT_OPTS" | grep -qE '\bro\b|readonly'; then
        log_error "✗ Mount is READ-ONLY"
        echo ""
        log_info "To fix, you need to:"
        echo "  1. Check LXC container config on HOST:"
        echo "     sudo cat /var/lib/lxc/$CONTAINER_NAME/config"
        echo "  2. Look for bind mount entry like:"
        echo "     lxc.mount.entry = /host/path /main-library-2 none bind,ro"
        echo "  3. Change 'ro' to 'rw' or remove it:"
        echo "     lxc.mount.entry = /host/path /main-library-2 none bind,rw"
        echo "  4. Restart container from HOST:"
        echo "     sudo lxc-stop -n $CONTAINER_NAME"
        echo "     sudo lxc-start -n $CONTAINER_NAME"
    else
        log_info "✓ Mount appears to be read-write"
    fi
else
    log_warn "Could not find mount info for /main-library-2"
fi
echo ""

# Check if we can actually write
log_info "Testing write access:"
TEST_FILE="/main-library-2/Media/Movies/.lxc-write-test-$$"
if touch "$TEST_FILE" 2>/dev/null; then
    log_info "✓ Can create files (as current user)"
    rm -f "$TEST_FILE"
    
    # Test as av1d
    if sudo -u av1d touch "$TEST_FILE" 2>/dev/null; then
        log_info "✓ av1d user can create files"
        sudo -u av1d rm -f "$TEST_FILE"
    else
        log_error "✗ av1d user CANNOT create files"
        echo "  Error: $(sudo -u av1d touch "$TEST_FILE" 2>&1)"
    fi
else
    log_error "✗ Cannot create files"
    echo "  Error: $(touch "$TEST_FILE" 2>&1)"
fi
echo ""

# Check LXC config location (if accessible)
log_info "Checking for LXC config files:"
if [ -f /var/lib/lxc/"$CONTAINER_NAME"/config ] 2>/dev/null; then
    log_info "Found config: /var/lib/lxc/$CONTAINER_NAME/config"
    echo "  Checking for /main-library-2 mount entry:"
    grep -i "main-library" /var/lib/lxc/"$CONTAINER_NAME"/config 2>/dev/null || echo "  (not found in config)"
elif [ -f /etc/lxc/"$CONTAINER_NAME".conf ] 2>/dev/null; then
    log_info "Found config: /etc/lxc/$CONTAINER_NAME.conf"
    echo "  Checking for /main-library-2 mount entry:"
    grep -i "main-library" /etc/lxc/"$CONTAINER_NAME".conf 2>/dev/null || echo "  (not found in config)"
else
    log_warn "LXC config not found in container"
    echo "  Config must be edited on the HOST system"
fi
echo ""

log_info "=== Instructions for HOST System ==="
echo ""
echo "On the HOST (not in container), run:"
echo ""
echo "1. Find container name:"
echo "   sudo lxc-ls -f"
echo ""
echo "2. Check current mount config:"
echo "   sudo cat /var/lib/lxc/CONTAINER_NAME/config | grep -i 'main-library'"
echo ""
echo "3. Edit config to make mount read-write:"
echo "   sudo nano /var/lib/lxc/CONTAINER_NAME/config"
echo ""
echo "4. Find line like:"
echo "   lxc.mount.entry = /host/path /main-library-2 none bind,ro"
echo ""
echo "5. Change to:"
echo "   lxc.mount.entry = /host/path /main-library-2 none bind,rw"
echo ""
echo "6. Restart container:"
echo "   sudo lxc-stop -n CONTAINER_NAME"
echo "   sudo lxc-start -n CONTAINER_NAME"
echo ""
echo "7. Verify in container:"
echo "   mount | grep '/main-library-2'"
echo "   (should show 'rw' not 'ro')"

