#!/bin/bash
# Update systemd service ReadWritePaths with library roots from config

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

CONFIG_FILE="/etc/av1qsvd/config.json"
SERVICE_FILE="/etc/systemd/system/av1d.service"

log_info "=== Updating Service ReadWritePaths ==="
echo ""

# Read library roots from config
if [ ! -f "$CONFIG_FILE" ]; then
    log_error "Config file not found: $CONFIG_FILE"
    exit 1
fi

# Extract library roots using python or jq if available
if command -v jq &>/dev/null; then
    LIB_ROOTS=$(jq -r '.library_roots[]' "$CONFIG_FILE" 2>/dev/null)
elif command -v python3 &>/dev/null; then
    LIB_ROOTS=$(python3 -c "import json; [print(r) for r in json.load(open('$CONFIG_FILE'))['library_roots']]" 2>/dev/null)
else
    # Fallback: grep and parse (basic)
    LIB_ROOTS=$(grep -o '"library_roots":\s*\[.*\]' "$CONFIG_FILE" | grep -o '"[^"]*"' | tr -d '"' | grep -v library_roots)
fi

if [ -z "$LIB_ROOTS" ]; then
    log_warn "No library roots found in config"
    log_info "Adding /main-library-2 as default"
    LIB_ROOTS="/main-library-2"
fi

log_info "Library roots from config:"
echo "$LIB_ROOTS" | while read root; do
    echo "  - $root"
done
echo ""

# Build ReadWritePaths
BASE_PATHS="/var/lib/av1qsvd /dev/dri"
ALL_PATHS="$BASE_PATHS"
echo "$LIB_ROOTS" | while read root; do
    if [ -n "$root" ]; then
        ALL_PATHS="$ALL_PATHS $root"
    fi
done

# Update service file
if [ ! -f "$SERVICE_FILE" ]; then
    log_error "Service file not found: $SERVICE_FILE"
    exit 1
fi

log_info "Updating service file..."
cp "$SERVICE_FILE" "${SERVICE_FILE}.bak"
log_info "Backup created: ${SERVICE_FILE}.bak"

# Build the new ReadWritePaths line
NEW_PATHS="/var/lib/av1qsvd /dev/dri"
echo "$LIB_ROOTS" | while read root; do
    if [ -n "$root" ]; then
        NEW_PATHS="$NEW_PATHS $root"
    fi
done

# Update the ReadWritePaths line
if grep -q "^ReadWritePaths=" "$SERVICE_FILE"; then
    # Replace existing line
    sed -i "s|^ReadWritePaths=.*|ReadWritePaths=$NEW_PATHS|" "$SERVICE_FILE"
    log_info "✓ Updated ReadWritePaths"
else
    # Add new line after ProtectHome
    sed -i "/^ProtectHome=/a ReadWritePaths=$NEW_PATHS" "$SERVICE_FILE"
    log_info "✓ Added ReadWritePaths"
fi

log_info "New ReadWritePaths:"
grep "^ReadWritePaths=" "$SERVICE_FILE"
echo ""

log_info "=== Next Steps ==="
echo "1. Reload systemd:"
echo "   sudo systemctl daemon-reload"
echo ""
echo "2. Restart service:"
echo "   sudo systemctl restart av1d"
echo ""
echo "3. Verify:"
echo "   sudo journalctl -u av1d -f"

