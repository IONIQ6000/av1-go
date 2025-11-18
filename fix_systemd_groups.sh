#!/bin/bash
# Fix systemd service to include media group

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

SERVICE_FILE="/etc/systemd/system/av1d.service"

log_info "=== Fixing Systemd Service Groups ==="
echo ""

if [ ! -f "$SERVICE_FILE" ]; then
    log_error "Service file not found: $SERVICE_FILE"
    exit 1
fi

log_info "Current service file:"
grep -A 2 "SupplementaryGroups" "$SERVICE_FILE" || log_warn "SupplementaryGroups not found"
echo ""

# Check if media is already in the list
if grep -q "SupplementaryGroups.*media" "$SERVICE_FILE" || grep -q "SupplementaryGroups=.*media" "$SERVICE_FILE"; then
    log_info "✓ 'media' group already in SupplementaryGroups"
else
    log_warn "⚠ 'media' group NOT in SupplementaryGroups"
    log_info "Updating service file..."
    
    # Backup original
    cp "$SERVICE_FILE" "${SERVICE_FILE}.bak"
    log_info "Backup created: ${SERVICE_FILE}.bak"
    
    # Update SupplementaryGroups line
    if grep -q "SupplementaryGroups=video render" "$SERVICE_FILE"; then
        sed -i 's/SupplementaryGroups=video render/SupplementaryGroups=video render media/' "$SERVICE_FILE"
        log_info "✓ Updated: SupplementaryGroups=video render media"
    elif grep -q "SupplementaryGroups=" "$SERVICE_FILE"; then
        # If it exists but in different format, update it
        sed -i 's/^SupplementaryGroups=\(.*\)$/SupplementaryGroups=\1 media/' "$SERVICE_FILE"
        log_info "✓ Added 'media' to existing SupplementaryGroups"
    else
        # If it doesn't exist, add it after Group line
        sed -i '/^Group=/a SupplementaryGroups=video render media' "$SERVICE_FILE"
        log_info "✓ Added: SupplementaryGroups=video render media"
    fi
    
    echo ""
    log_info "Updated service file:"
    grep -A 2 "SupplementaryGroups" "$SERVICE_FILE"
    echo ""
    
    # Reload systemd
    log_info "Reloading systemd..."
    systemctl daemon-reload
    log_info "✓ Systemd reloaded"
    echo ""
    
    log_warn "⚠ Service needs to be restarted for changes to take effect"
fi
echo ""

log_info "=== Next Steps ==="
echo "1. Restart the service:"
echo "   sudo systemctl stop av1d"
echo "   sudo systemctl start av1d"
echo ""
echo "2. Verify process has correct groups:"
echo "   sudo ./test_service_groups.sh"
echo ""
echo "3. Monitor logs:"
echo "   sudo journalctl -u av1d -f"

