#!/bin/bash

# Configuration diagnostic script

echo "=== AV1 Daemon Configuration Check ==="
echo ""

# Check config file
CONFIG_FILE="/etc/av1qsvd/config.json"
if [ -f "$CONFIG_FILE" ]; then
    echo "✓ Config file exists: $CONFIG_FILE"
    echo ""
    echo "Current configuration:"
    cat "$CONFIG_FILE" | python3 -m json.tool 2>/dev/null || cat "$CONFIG_FILE"
    echo ""
else
    echo "✗ Config file NOT found: $CONFIG_FILE"
    echo "  Creating default config..."
    sudo mkdir -p /etc/av1qsvd
    sudo tee "$CONFIG_FILE" > /dev/null <<EOF
{
  "ffmpeg_url": "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n8.0-latest-linux64-gpl-8.0.tar.xz",
  "ffmpeg_install_dir": "/var/lib/av1qsvd/ffmpeg",
  "library_roots": [],
  "min_bytes": 2147483648,
  "max_size_ratio": 0.90,
  "job_state_dir": "/var/lib/av1qsvd/jobs",
  "scan_interval_sec": 60
}
EOF
    echo "  Default config created. Edit it to add library_roots:"
    echo "  sudo nano $CONFIG_FILE"
    echo ""
fi

# Check library roots
echo "=== Library Roots Check ==="
LIB_ROOTS=$(cat "$CONFIG_FILE" 2>/dev/null | grep -o '"library_roots":\s*\[.*\]' | grep -o '\[.*\]' || echo "[]")
if [ "$LIB_ROOTS" == "[]" ]; then
    echo "✗ No library roots configured!"
    echo ""
    echo "To add library roots, edit the config:"
    echo "  sudo nano $CONFIG_FILE"
    echo ""
    echo "Add paths to library_roots array, e.g.:"
    echo '  "library_roots": ["/path/to/media", "/another/path"]'
    echo ""
else
    echo "✓ Library roots configured:"
    echo "$LIB_ROOTS"
    echo ""
    
    # Check if directories exist
    echo "Checking if directories exist..."
    echo "$LIB_ROOTS" | grep -o '"[^"]*"' | tr -d '"' | while read -r dir; do
        if [ -d "$dir" ]; then
            echo "  ✓ $dir exists"
            # Count media files
            count=$(find "$dir" -type f \( -iname "*.mkv" -o -iname "*.mp4" -o -iname "*.m4v" \) 2>/dev/null | wc -l)
            echo "    Found $count media files"
        else
            echo "  ✗ $dir does NOT exist"
        fi
    done
fi

echo ""
echo "=== Service Status ==="
if systemctl is-active --quiet av1d; then
    echo "✓ Service is running"
    echo ""
    echo "Recent logs:"
    sudo journalctl -u av1d -n 20 --no-pager
else
    echo "✗ Service is not running"
    echo "  Start with: sudo systemctl start av1d"
fi

echo ""
echo "=== Quick Test ==="
echo "To test scanning manually, run:"
echo "  sudo /usr/local/bin/av1d"
echo ""
echo "This will show verbose output of what files are found and why they're skipped/accepted."

