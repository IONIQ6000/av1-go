#!/bin/bash
# Auto-fix GPU access for av1d user after container restart
# This script runs on boot to ensure GPU access is available

set -e

# Add av1d to required groups (idempotent)
if id av1d &>/dev/null; then
    usermod -a -G video,render,media av1d 2>/dev/null || true
fi

# Ensure GPU devices are accessible
if [ -d "/dev/dri" ]; then
    for device in /dev/dri/renderD*; do
        if [ -e "$device" ]; then
            # Try to set permissions (may require host-side udev rules)
            chmod 666 "$device" 2>/dev/null || true
        fi
    done
fi

exit 0

