#!/bin/bash
# Test if av1d user can write to media directories

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

log_info "=== Testing av1d User Write Access ==="
echo ""

# Test a few directories from the error logs
TEST_DIRS=(
    "/main-library-2/Media/Movies/Terminator 2 Judgment Day (1991)"
    "/main-library-2/Media/Movies/Oppenheimer (2023) {tmdb-872585}"
    "/main-library-2/Media/Movies/Ex Machina (2015) {tmdb-264660}"
    "/main-library-2/Media/Movies/RRR (2022)"
    "/main-library-2/Media/Movies/The Last Emperor (1987) {tmdb-746}"
)

for dir in "${TEST_DIRS[@]}"; do
    if [ -d "$dir" ]; then
        log_info "Testing: $dir"
        
        # Check ownership
        OWNER=$(stat -c '%U:%G' "$dir" 2>/dev/null || stat -f '%Su:%Sg' "$dir" 2>/dev/null)
        PERMS=$(stat -c '%a' "$dir" 2>/dev/null || stat -f '%OLp' "$dir" 2>/dev/null)
        echo "  Owner: $OWNER, Perms: $PERMS"
        
        # Try to create a test file as av1d user
        TEST_FILE="$dir/.av1d-write-test-$$"
        if sudo -u av1d touch "$TEST_FILE" 2>/dev/null; then
            log_info "  ✓ av1d can create files"
            sudo -u av1d rm -f "$TEST_FILE" 2>/dev/null
            
            # Try to create the actual temp file name
            TMP_FILE="$dir/test.av1-tmp.mkv"
            if sudo -u av1d touch "$TMP_FILE" 2>/dev/null; then
                log_info "  ✓ av1d can create .av1-tmp.mkv files"
                sudo -u av1d rm -f "$TMP_FILE" 2>/dev/null
            else
                log_error "  ✗ av1d CANNOT create .av1-tmp.mkv files"
                echo "    Error: $(sudo -u av1d touch "$TMP_FILE" 2>&1)"
            fi
        else
            log_error "  ✗ av1d CANNOT create files"
            echo "    Error: $(sudo -u av1d touch "$TEST_FILE" 2>&1)"
        fi
        echo ""
    fi
done

# Check ZFS dataset properties
log_info "Checking ZFS dataset properties:"
if command -v zfs &>/dev/null; then
    DATASET=$(zfs list -o name,mountpoint | grep "/main-library-2" | awk '{print $1}' | head -1)
    if [ -n "$DATASET" ]; then
        log_info "Dataset: $DATASET"
        READONLY=$(zfs get -H -o value readonly "$DATASET" 2>/dev/null)
        CANMOUNT=$(zfs get -H -o value canmount "$DATASET" 2>/dev/null)
        echo "  readonly: $READONLY"
        echo "  canmount: $CANMOUNT"
        
        if [ "$READONLY" = "on" ]; then
            log_error "✗ ZFS dataset is set to readonly=on"
            echo "  Fix: sudo zfs set readonly=off $DATASET"
        else
            log_info "✓ ZFS dataset is not readonly"
        fi
    else
        log_warn "Could not find ZFS dataset for /main-library-2"
    fi
else
    log_warn "zfs command not available"
fi
echo ""

log_info "=== Recommendations ==="
echo "If av1d still can't write after chown:"
echo "  1. Check ZFS dataset: sudo zfs get readonly wd-dell-disk/main-library-2"
echo "  2. If readonly=on: sudo zfs set readonly=off wd-dell-disk/main-library-2"
echo "  3. Restart service: sudo systemctl restart av1d"
echo "  4. Check ACLs: getfacl /main-library-2/Media/Movies"
echo "  5. Verify ownership recursively: sudo chown -R av1d:av1d /main-library-2/Media"

