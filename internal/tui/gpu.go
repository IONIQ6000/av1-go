package tui

import (
	"os"
	"os/exec"
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

// getGPUUsageFromSysfs calculates GPU usage from engine utilization (more accurate than frequency)
func getGPUUsageFromSysfs(cardPath string) float64 {
	// Method 1: Read engine busy percentages (most accurate)
	// Path: /sys/class/drm/cardX/gt/gt0/engines/*/busy
	gtPath := filepath.Join(cardPath, "gt", "gt0", "engines")
	if engines, err := os.ReadDir(gtPath); err == nil {
		var totalBusy, count float64
		for _, engine := range engines {
			if !engine.IsDir() {
				continue
			}
			busyPath := filepath.Join(gtPath, engine.Name(), "busy")
			if data, err := os.ReadFile(busyPath); err == nil {
				// Busy is typically a percentage (0-100) or a ratio
				busyStr := strings.TrimSpace(string(data))
				if busy, err := strconv.ParseFloat(busyStr, 64); err == nil {
					// If it's already a percentage (0-100), use it directly
					// If it's a ratio (0-1), multiply by 100
					if busy <= 1.0 {
						busy = busy * 100.0
					}
					totalBusy += busy
					count++
				}
			}
		}
		if count > 0 {
			avgBusy := totalBusy / count
			if avgBusy > 0 {
				return avgBusy
			}
		}
	}

	// Method 2: Read from intel_gpu_frequency (if available)
	freqPath := filepath.Join(cardPath, "gt", "gt0", "intel_gpu_freq")
	if data, err := os.ReadFile(freqPath); err == nil {
		// Format: "act: 1200 MHz, req: 1200 MHz, idle: 200 MHz"
		freqStr := strings.ToLower(string(data))
		if strings.Contains(freqStr, "act:") {
			// Try to extract actual frequency
			parts := strings.Split(freqStr, "act:")
			if len(parts) > 1 {
				freqPart := strings.Split(parts[1], "mhz")[0]
				freqPart = strings.TrimSpace(freqPart)
				if actFreq, err := strconv.ParseFloat(freqPart, 64); err == nil {
					// Try to find max frequency
					maxFreqPath := filepath.Join(cardPath, "gt", "gt0", "rps_max_freq_mhz")
					if maxData, err := os.ReadFile(maxFreqPath); err == nil {
						if maxFreq, err := strconv.ParseFloat(strings.TrimSpace(string(maxData)), 64); err == nil && maxFreq > 0 {
							usage := (actFreq / maxFreq) * 100.0
							if usage > 100.0 {
								usage = 100.0
							}
							return usage
						}
					}
				}
			}
		}
	}

	// Method 3: Fallback to frequency-based calculation
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

	usage := (float64(actFreq) / float64(maxFreq)) * 100.0
	if usage > 100.0 {
		usage = 100.0
	}
	return usage
}

// getGPUUsageFromIntelGPUTop tries to get GPU usage from intel_gpu_top command
func getGPUUsageFromIntelGPUTop() float64 {
	// Try to run intel_gpu_top with -l flag (one-shot, no interactive)
	// Command: intel_gpu_top -l -n 1
	// This outputs one snapshot and exits
	cmd := exec.Command("intel_gpu_top", "-l", "-n", "1")
	output, err := cmd.Output()
	if err != nil {
		return 0.0
	}

	// Parse output - intel_gpu_top shows engine utilization
	// Look for lines like "RC6: X%", "Render/3D: X%", "Video/0: X%", etc.
	lines := strings.Split(string(output), "\n")
	var totalUtil, count float64
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for percentage patterns: "Render/3D: 45.2%", "Video/0: 12.3%", etc.
		if strings.Contains(line, "%") {
			// Try to extract percentage
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasSuffix(part, "%") {
					percentStr := strings.TrimSuffix(part, "%")
					if util, err := strconv.ParseFloat(percentStr, 64); err == nil {
						// Only count non-zero, reasonable values
						if util > 0 && util <= 100 {
							totalUtil += util
							count++
						}
					}
				}
			}
		}
	}
	
	if count > 0 {
		avgUtil := totalUtil / count
		return avgUtil
	}
	
	return 0.0
}

