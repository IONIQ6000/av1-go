#!/bin/bash
# Check if the running service has the correct group membership

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

log_info "=== Checking Service Process Groups ==="
echo ""

# Find av1d process
AV1D_PID=$(pgrep -f "av1d" | head -1)
if [ -z "$AV1D_PID" ]; then
    log_error "av1d process not found"
    exit 1
fi

log_info "Found av1d process: PID $AV1D_PID"
echo ""

# Check what user it's running as
PROC_USER=$(ps -o user= -p "$AV1D_PID" | tr -d ' ')
log_info "Process running as user: $PROC_USER"
echo ""

# Check groups of the running process
log_info "Groups of running process:"
if [ -f "/proc/$AV1D_PID/status" ]; then
    echo "  UID: $(grep ^Uid: /proc/$AV1D_PID/status)"
    echo "  GID: $(grep ^Gid: /proc/$AV1D_PID/status)"
    echo ""
    
    # Get actual group list
    GROUPS=$(cat /proc/$AV1D_PID/status | grep ^Groups: | cut -f2)
    log_info "Group IDs: $GROUPS"
    
    # Check if GID 568 is in the list
    if echo "$GROUPS" | grep -q "568"; then
        log_info "✓ Process has GID 568 in groups"
    else
        log_error "✗ Process does NOT have GID 568 in groups"
        log_warn "  Service needs to be restarted to pick up new group membership"
    fi
else
    log_warn "Cannot read /proc/$AV1D_PID/status"
fi
echo ""

# Check av1d user's current groups (what new processes will get)
log_info "av1d user groups (for new processes):"
groups av1d
echo ""

# Check if media group (568) is in the list
if groups av1d | grep -q "media"; then
    log_info "✓ av1d user is in 'media' group"
else
    log_error "✗ av1d user is NOT in 'media' group"
    log_info "  Adding av1d to media group..."
    sudo usermod -aG media av1d
    log_info "  ✓ Added (service restart required)"
fi
echo ""

# Find ffmpeg path
log_info "Finding ffmpeg binary:"
FFMPEG_PATH=""
if [ -f "/home/av1d/.local/share/av1qsvd/ffmpeg/ffmpeg" ]; then
    FFMPEG_PATH="/home/av1d/.local/share/av1qsvd/ffmpeg/ffmpeg"
elif [ -f "/var/lib/av1d/.local/share/av1qsvd/ffmpeg/ffmpeg" ]; then
    FFMPEG_PATH="/var/lib/av1d/.local/share/av1qsvd/ffmpeg/ffmpeg"
else
    FFMPEG_PATH=$(find /home/av1d /var/lib/av1d -name ffmpeg 2>/dev/null | head -1)
fi

if [ -n "$FFMPEG_PATH" ] && [ -f "$FFMPEG_PATH" ]; then
    log_info "Found: $FFMPEG_PATH"
    
    # Check ffmpeg permissions
    FFMPEG_OWNER=$(stat -c '%U:%G' "$FFMPEG_PATH" 2>/dev/null || stat -f '%Su:%Sg' "$FFMPEG_PATH" 2>/dev/null)
    FFMPEG_PERMS=$(stat -c '%a' "$FFMPEG_PATH" 2>/dev/null || stat -f '%OLp' "$FFMPEG_PATH" 2>/dev/null)
    echo "  Owner: $FFMPEG_OWNER"
    echo "  Permissions: $FFMPEG_PERMS"
    echo ""
    
    # Test ffmpeg write as av1d
    log_info "Testing ffmpeg write capability:"
    TEST_DIR="/main-library-2/Media/Movies"
    TEST_OUTPUT="$TEST_DIR/.ffmpeg-direct-test-$$.mkv"
    
    # Run ffmpeg as av1d with same environment as service
    log_info "Running: sudo -u av1d $FFMPEG_PATH -f lavfi -i testsrc=duration=1:size=320x240 -c:v libx264 -t 1 \"$TEST_OUTPUT\""
    if sudo -u av1d "$FFMPEG_PATH" -f lavfi -i testsrc=duration=1:size=320x240 -c:v libx264 -t 1 "$TEST_OUTPUT" 2>&1 | tail -5; then
        if [ -f "$TEST_OUTPUT" ]; then
            log_info "✓ FFmpeg successfully created output file!"
            sudo -u av1d rm -f "$TEST_OUTPUT"
        else
            log_error "✗ FFmpeg ran but file was not created"
        fi
    else
        ERROR_OUTPUT=$(sudo -u av1d "$FFMPEG_PATH" -f lavfi -i testsrc=duration=1:size=320x240 -c:v libx264 -t 1 "$TEST_OUTPUT" 2>&1)
        if echo "$ERROR_OUTPUT" | grep -q "Read-only file system"; then
            log_error "✗ FFmpeg reports 'Read-only file system'"
            echo "  This confirms the issue"
        else
            log_warn "FFmpeg test had errors (may be expected for test):"
            echo "$ERROR_OUTPUT" | tail -10
        fi
    fi
else
    log_warn "Could not find ffmpeg binary"
    echo "  Searched in: /home/av1d, /var/lib/av1d"
fi
echo ""

log_info "=== Fix Steps ==="
echo "1. Ensure av1d is in media group:"
echo "   sudo usermod -aG media av1d"
echo ""
echo "2. FULLY restart service (stop + start, not just restart):"
echo "   sudo systemctl stop av1d"
echo "   sleep 2"
echo "   sudo systemctl start av1d"
echo ""
echo "3. Verify process has correct groups:"
echo "   sudo ./test_service_groups.sh"
echo ""
echo "4. Monitor logs:"
echo "   sudo journalctl -u av1d -f"

