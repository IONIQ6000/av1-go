#!/bin/bash
# Fix container permissions - ensure av1d user is in the correct group

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

log_info "=== Fixing Container Permissions ==="
echo ""

# Check what group has GID 568
log_info "Checking group with GID 568:"
GROUP_568=$(getent group 568 | cut -d: -f1)
if [ -n "$GROUP_568" ]; then
    log_info "✓ Found group: $GROUP_568 (GID 568)"
else
    log_warn "⚠ No group found with GID 568"
    echo "  Creating group 'media' with GID 568..."
    sudo groupadd -g 568 media 2>/dev/null || log_warn "Group may already exist"
    GROUP_568="media"
fi
echo ""

# Check av1d user's current groups
log_info "Checking av1d user groups:"
AV1D_GROUPS=$(groups av1d | cut -d: -f2 | tr ' ' '\n')
echo "  Current groups: $(groups av1d)"
echo ""

# Check if av1d is in the GID 568 group
if echo "$AV1D_GROUPS" | grep -q "^${GROUP_568}$"; then
    log_info "✓ av1d is already in group $GROUP_568"
else
    log_warn "⚠ av1d is NOT in group $GROUP_568"
    log_info "Adding av1d to group $GROUP_568..."
    sudo usermod -aG "$GROUP_568" av1d
    log_info "✓ Added av1d to group $GROUP_568"
    echo ""
    log_warn "⚠ Service needs to be restarted for group changes to take effect"
fi
echo ""

# Check file permissions on a sample directory
log_info "Checking file permissions:"
SAMPLE_DIR="/main-library-2/Media/Movies"
if [ -d "$SAMPLE_DIR" ]; then
    SAMPLE_FILE=$(find "$SAMPLE_DIR" -type f -name "*.mkv" | head -1)
    if [ -n "$SAMPLE_FILE" ]; then
        FILE_OWNER=$(stat -c '%U:%G' "$SAMPLE_FILE" 2>/dev/null || stat -f '%Su:%Sg' "$SAMPLE_FILE" 2>/dev/null)
        FILE_GID=$(stat -c '%g' "$SAMPLE_FILE" 2>/dev/null || stat -f '%g' "$SAMPLE_FILE" 2>/dev/null)
        FILE_PERMS=$(stat -c '%a' "$SAMPLE_FILE" 2>/dev/null || stat -f '%OLp' "$SAMPLE_FILE" 2>/dev/null)
        
        log_info "Sample file: $(basename "$SAMPLE_FILE")"
        echo "  Owner: $FILE_OWNER"
        echo "  GID: $FILE_GID"
        echo "  Permissions: $FILE_PERMS"
        
        if [ "$FILE_GID" = "568" ]; then
            log_info "✓ File has GID 568 (matches shared group)"
            
            # Check if group has write permission
            GROUP_PERM=$(echo "$FILE_PERMS" | cut -c2)
            if [ "$GROUP_PERM" -ge 6 ]; then
                log_info "✓ Group has write permission (6 or 7)"
            else
                log_warn "⚠ Group does NOT have write permission"
                echo "  Current: $FILE_PERMS"
                echo "  Need: 6xx or 7xx for group write"
                echo ""
                log_info "To fix, run on HOST (not container):"
                echo "  sudo chmod -R g+w /host/path/to/library"
            fi
        else
            log_warn "⚠ File GID ($FILE_GID) does not match expected 568"
        fi
    fi
fi
echo ""

# Verify av1d can now write
log_info "Testing write access as av1d:"
TEST_FILE="/main-library-2/Media/Movies/.av1d-perm-test-$$"
if sudo -u av1d touch "$TEST_FILE" 2>/dev/null; then
    log_info "✓ av1d can create files"
    sudo -u av1d rm -f "$TEST_FILE"
    
    # Test the actual temp file
    TMP_FILE="/main-library-2/Media/Movies/test.av1-tmp.mkv"
    if sudo -u av1d touch "$TMP_FILE" 2>/dev/null; then
        log_info "✓ av1d can create .av1-tmp.mkv files"
        sudo -u av1d rm -f "$TMP_FILE"
    else
        log_error "✗ Cannot create .av1-tmp.mkv files"
    fi
else
    log_error "✗ av1d still cannot create files"
    echo "  Error: $(sudo -u av1d touch "$TEST_FILE" 2>&1)"
fi
echo ""

log_info "=== Next Steps ==="
echo "1. Restart av1d service:"
echo "   sudo systemctl restart av1d"
echo ""
echo "2. Monitor logs:"
echo "   sudo journalctl -u av1d -f"
echo ""
echo "3. If still failing, check on HOST:"
echo "   - Ensure files have group write: chmod -R g+w /host/path"
echo "   - Verify GID 568 group exists on host"
echo "   - Check that container's GID 568 group matches host"

