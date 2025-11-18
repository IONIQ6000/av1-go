package tui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// getGPUUsage attempts to get Intel GPU utilization percentage.
// Returns 0.0 if unable to determine GPU usage.
func getGPUUsage() float64 {
	// Try to find Intel GPU card
	cardPath := findIntelGPUCard()
	if cardPath == "" {
		return 0.0
	}

	// Try multiple methods to get GPU utilization
	
	// Method 1: Read from intel_gpu_top if available (most accurate)
	if usage := getGPUUsageFromIntelGPUTop(); usage > 0 {
		return usage
	}

	// Method 2: Read from sysfs frequency-based utilization
	if usage := getGPUUsageFromSysfs(cardPath); usage > 0 {
		return usage
	}

	return 0.0
}

// findIntelGPUCard finds the first Intel GPU card in /sys/class/drm
func findIntelGPUCard() string {
	drmPath := "/sys/class/drm"
	entries, err := os.ReadDir(drmPath)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "card") {
			continue
		}

		// Check if it's an Intel GPU by looking for vendor file
		vendorPath := filepath.Join(drmPath, name, "device", "vendor")
		vendorData, err := os.ReadFile(vendorPath)
		if err != nil {
			continue
		}

		// Intel vendor ID is 0x8086
		if strings.Contains(strings.ToLower(string(vendorData)), "8086") {
			return filepath.Join(drmPath, name, "device")
		}
	}

	return ""
}

// getGPUUsageFromSysfs calculates GPU usage based on frequency
func getGPUUsageFromSysfs(cardPath string) float64 {
	// Try to read current and max frequency
	// Path varies by kernel version, try multiple locations
	
	// Try gt/gt0/rps_act_freq_mhz (current frequency)
	actFreqPaths := []string{
		filepath.Join(cardPath, "gt", "gt0", "rps_act_freq_mhz"),
		filepath.Join(cardPath, "gt_act_freq_mhz"),
		filepath.Join(cardPath, "gt_cur_freq_mhz"),
	}
	
	var actFreq int64
	for _, path := range actFreqPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		val, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err == nil && val > 0 {
			actFreq = val
			break
		}
	}

	if actFreq == 0 {
		return 0.0
	}

	// Try to read max frequency
	maxFreqPaths := []string{
		filepath.Join(cardPath, "gt", "gt0", "rps_max_freq_mhz"),
		filepath.Join(cardPath, "gt_max_freq_mhz"),
		filepath.Join(cardPath, "gt_boost_freq_mhz"),
	}
	
	var maxFreq int64
	for _, path := range maxFreqPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		val, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err == nil && val > 0 {
			maxFreq = val
			break
		}
	}

	if maxFreq == 0 {
		return 0.0
	}

	// Calculate utilization as percentage of max frequency
	usage := (float64(actFreq) / float64(maxFreq)) * 100.0
	if usage > 100.0 {
		usage = 100.0
	}
	return usage
}

// getGPUUsageFromIntelGPUTop tries to get GPU usage from intel_gpu_top command
func getGPUUsageFromIntelGPUTop() float64 {
	// This would require parsing intel_gpu_top output
	// For now, return 0 to use sysfs method
	// Future enhancement: parse intel_gpu_top -l output
	return 0.0
}

