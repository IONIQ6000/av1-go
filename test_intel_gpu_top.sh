#!/bin/bash
# Test intel_gpu_top output format

echo "=== Testing intel_gpu_top output ==="
echo ""
echo "Running: timeout 1 intel_gpu_top -l -s 500"
echo ""

timeout 1 intel_gpu_top -l -s 500 2>&1 | head -30

echo ""
echo "=== Looking for percentage patterns ==="
timeout 1 intel_gpu_top -l -s 500 2>&1 | grep -E "%|Video|Render|Blitter" | head -10

