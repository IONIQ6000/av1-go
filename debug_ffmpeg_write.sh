#!/bin/bash
# Debug why ffmpeg can't write even though touch works

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

log_info "=== Debugging FFmpeg Write Access ==="
echo ""

# Test the exact directories from the error logs
FAILING_DIRS=(
    "/main-library-2/Media/Movies/Batman Begins (2005) {tmdb-272}"
    "/main-library-2/Media/Movies/Hellbender (2021)"
    "/main-library-2/Media/Movies/GoodFellas (1990)"
)

for dir in "${FAILING_DIRS[@]}"; do
    if [ -d "$dir" ]; then
        log_info "Checking: $dir"
        
        # Check directory ownership and permissions
        DIR_OWNER=$(stat -c '%U:%G' "$dir" 2>/dev/null || stat -f '%Su:%Sg' "$dir" 2>/dev/null)
        DIR_GID=$(stat -c '%g' "$dir" 2>/dev/null || stat -f '%g' "$dir" 2>/dev/null)
        DIR_PERMS=$(stat -c '%a' "$dir" 2>/dev/null || stat -f '%OLp' "$dir" 2>/dev/null)
        
        echo "  Owner: $DIR_OWNER"
        echo "  GID: $DIR_GID"
        echo "  Permissions: $DIR_PERMS"
        
        # Check if directory is writable
        if [ -w "$dir" ]; then
            log_info "  ✓ Directory is writable by current user"
        else
            log_error "  ✗ Directory is NOT writable by current user"
        fi
        
        # Test as av1d user
        TEST_FILE="$dir/.av1d-dir-test-$$"
        if sudo -u av1d touch "$TEST_FILE" 2>/dev/null; then
            log_info "  ✓ av1d can create files in this directory"
            sudo -u av1d rm -f "$TEST_FILE"
            
            # Test the exact filename pattern ffmpeg uses
            TMP_FILE="$dir/test.av1-tmp.mkv"
            if sudo -u av1d touch "$TMP_FILE" 2>/dev/null; then
                log_info "  ✓ av1d can create .av1-tmp.mkv in this directory"
                sudo -u av1d rm -f "$TMP_FILE"
            else
                log_error "  ✗ av1d CANNOT create .av1-tmp.mkv in this directory"
                echo "    Error: $(sudo -u av1d touch "$TMP_FILE" 2>&1)"
            fi
        else
            log_error "  ✗ av1d CANNOT create files in this directory"
            echo "    Error: $(sudo -u av1d touch "$TEST_FILE" 2>&1)"
        fi
        
        # Check if directory needs ownership/permission fix
        if [ "$DIR_OWNER" != "av1d:av1d" ] && [ "$DIR_OWNER" != "av1d:media" ]; then
            log_warn "  ⚠ Directory owned by $DIR_OWNER (not av1d)"
            echo "    Fix: sudo chown -R av1d:av1d \"$dir\""
        fi
        
        # Check group write permission
        GROUP_PERM=$(echo "$DIR_PERMS" | cut -c2)
        if [ "$GROUP_PERM" -lt 6 ]; then
            log_warn "  ⚠ Directory group does NOT have write permission"
            echo "    Current: $DIR_PERMS"
            echo "    Fix: sudo chmod g+w \"$dir\""
        fi
        
        echo ""
    else
        log_warn "Directory does not exist: $dir"
        echo ""
    fi
done

# Check mount options one more time
log_info "Checking mount options:"
MOUNT_INFO=$(mount | grep "/main-library-2")
if [ -n "$MOUNT_INFO" ]; then
    echo "$MOUNT_INFO"
    if echo "$MOUNT_INFO" | grep -qE '\bro\b|readonly'; then
        log_error "✗ Mount is READ-ONLY"
        echo "  This is the problem! Fix the LXC bind mount on the HOST."
    else
        log_info "✓ Mount is read-write"
    fi
fi
echo ""

# Test ffmpeg directly as av1d
log_info "Testing ffmpeg directly as av1d user:"
FFMPEG_PATH=$(find /home/av1d -name ffmpeg 2>/dev/null | head -1)
if [ -z "$FFMPEG_PATH" ]; then
    FFMPEG_PATH=$(find /var/lib/av1d -name ffmpeg 2>/dev/null | head -1)
fi
if [ -z "$FFMPEG_PATH" ]; then
    FFMPEG_PATH="/usr/local/bin/ffmpeg"
fi

if [ -f "$FFMPEG_PATH" ]; then
    log_info "Found ffmpeg: $FFMPEG_PATH"
    TEST_DIR="/main-library-2/Media/Movies"
    TEST_OUTPUT="$TEST_DIR/.ffmpeg-test-$$.mkv"
    
    log_info "Running ffmpeg test (will fail if write access is broken):"
    if sudo -u av1d "$FFMPEG_PATH" -f lavfi -i testsrc=duration=1:size=320x240 -c:v libx264 -t 1 "$TEST_OUTPUT" 2>&1 | head -20; then
        log_info "✓ FFmpeg can write files!"
        sudo -u av1d rm -f "$TEST_OUTPUT"
    else
        log_error "✗ FFmpeg CANNOT write files"
        echo "  This confirms the issue - ffmpeg itself can't write"
    fi
else
    log_warn "Could not find ffmpeg to test"
fi
echo ""

log_info "=== Summary ==="
echo "If directories have wrong ownership:"
echo "  sudo chown -R av1d:av1d /main-library-2/Media/Movies"
echo ""
echo "If mount is read-only (check on HOST):"
echo "  sudo lxc-stop -n CONTAINER_NAME"
echo "  Edit: /var/lib/lxc/CONTAINER_NAME/config"
echo "  Change bind mount from 'ro' to 'rw'"
echo "  sudo lxc-start -n CONTAINER_NAME"

