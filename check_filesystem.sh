#!/bin/bash
# Check filesystem mount status and permissions for media library

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

log_info "=== Filesystem Check for Media Library ==="
echo ""

# Check mount status
log_info "Checking mount status for /main-library-2:"
if mount | grep -q "/main-library-2"; then
    mount | grep "/main-library-2"
    MOUNT_OPTS=$(mount | grep "/main-library-2" | grep -oP '\([^)]+\)' | tr -d '()')
    if echo "$MOUNT_OPTS" | grep -q "ro," || echo "$MOUNT_OPTS" | grep -q ",ro" || echo "$MOUNT_OPTS" | grep -q "^ro$"; then
        log_error "✗ Filesystem is mounted READ-ONLY"
        echo "  Remount with: sudo mount -o remount,rw /main-library-2"
    else
        log_info "✓ Filesystem is mounted read-write"
    fi
else
    log_warn "⚠ /main-library-2 not found in mount table"
    echo "  It might be a symlink or part of another mount"
fi
echo ""

# Check if directory exists and is writable
log_info "Checking directory permissions:"
TEST_DIR="/main-library-2/Media/Movies"
if [ -d "$TEST_DIR" ]; then
    log_info "✓ Directory exists: $TEST_DIR"
    
    # Check if writable by current user
    if [ -w "$TEST_DIR" ]; then
        log_info "✓ Directory is writable by current user"
    else
        log_error "✗ Directory is NOT writable by current user"
        echo "  Current user: $(whoami)"
        echo "  Directory owner: $(stat -c '%U:%G' "$TEST_DIR")"
        echo "  Permissions: $(stat -c '%a' "$TEST_DIR")"
    fi
    
    # Try to create a test file
    TEST_FILE="$TEST_DIR/.av1d-test-write-$$"
    if touch "$TEST_FILE" 2>/dev/null; then
        log_info "✓ Can create files in directory"
        rm -f "$TEST_FILE"
    else
        log_error "✗ Cannot create files in directory"
        echo "  Error: $(touch "$TEST_FILE" 2>&1)"
    fi
else
    log_error "✗ Directory does not exist: $TEST_DIR"
fi
echo ""

# Check av1d user permissions
log_info "Checking av1d service user:"
if id av1d &>/dev/null; then
    log_info "✓ av1d user exists"
    echo "  Groups: $(groups av1d)"
    
    # Check if av1d can write (if we can test as that user)
    if sudo -u av1d test -w "$TEST_DIR" 2>/dev/null; then
        log_info "✓ av1d user can write to directory"
    else
        log_warn "⚠ av1d user may not be able to write"
        echo "  Consider: sudo chown -R av1d:av1d $TEST_DIR"
        echo "  Or: sudo chmod -R g+w $TEST_DIR && sudo usermod -aG $(stat -c '%G' "$TEST_DIR") av1d"
    fi
else
    log_warn "⚠ av1d user does not exist"
fi
echo ""

log_info "=== Recommendations ==="
echo "If filesystem is read-only:"
echo "  1. Check /etc/fstab for mount options"
echo "  2. Remount: sudo mount -o remount,rw /main-library-2"
echo "  3. If it's an NFS/CIFS mount, check server-side permissions"
echo ""
echo "If permissions issue:"
echo "  1. Ensure av1d user is in the correct group"
echo "  2. Set directory permissions: sudo chmod -R g+w /main-library-2/Media"
echo "  3. Or change ownership: sudo chown -R av1d:media /main-library-2/Media"

