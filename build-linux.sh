#!/bin/bash
# Cross-compile for Linux from macOS

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_info "Cross-compiling for Linux (amd64)..."

# Set environment for Linux build
export GOOS=linux
export GOARCH=amd64

cd "$SCRIPT_DIR"

log_info "Building av1d..."
go build -o av1d-linux-amd64 ./cmd/av1d
log_info "✓ av1d built"

log_info "Building av1top..."
go build -o av1top-linux-amd64 ./cmd/av1top
log_info "✓ av1top built"

log_info ""
log_info "Binaries built:"
log_info "  - av1d-linux-amd64"
log_info "  - av1top-linux-amd64"
log_info ""
log_info "To install on server:"
log_info "  scp av1d-linux-amd64 av1top-linux-amd64 root@av1-go:~/av1-go/"
log_info "  ssh root@av1-go"
log_info "  cd ~/av1-go"
log_info "  sudo systemctl stop av1d"
log_info "  sudo cp av1d-linux-amd64 /usr/local/bin/av1d"
log_info "  sudo cp av1top-linux-amd64 /usr/local/bin/av1top"
log_info "  sudo systemctl start av1d"

