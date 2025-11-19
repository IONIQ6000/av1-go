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
// Returns the device path
func findIntelGPUCard() string {
	drmPath := "/sys/class/drm"
	entries, err := os.ReadDir(drmPath)
	if err != nil {
		return ""
	}

	// Try card0 first (usually primary), then card1, etc.
	cards := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "card") {
			continue
		}
		cards = append(cards, name)
	}

	// Sort to try card0 first
	for _, cardName := range cards {
		// Check if it's an Intel GPU by looking for vendor file
		vendorPath := filepath.Join(drmPath, cardName, "device", "vendor")
		vendorData, err := os.ReadFile(vendorPath)
		if err != nil {
			continue
		}

		// Intel vendor ID is 0x8086
		if strings.Contains(strings.ToLower(string(vendorData)), "8086") {
			return filepath.Join(drmPath, cardName, "device")
		}
	}

	return ""
}

// getGPUUsageFromSysfs calculates GPU usage from engine utilization (more accurate than frequency)
func getGPUUsageFromSysfs(cardPath string) float64 {
	// Method 1: Try multiple GT paths (gt0, gt1, etc.)
	// For Arc GPUs, the structure might be different
	gtBase := filepath.Join(cardPath, "gt")
	if gtDirs, err := os.ReadDir(gtBase); err == nil {
		for _, gtDir := range gtDirs {
			if !gtDir.IsDir() || !strings.HasPrefix(gtDir.Name(), "gt") {
				continue
			}
			
			// Try engines directory
			enginesPath := filepath.Join(gtBase, gtDir.Name(), "engines")
			if engines, err := os.ReadDir(enginesPath); err == nil {
				var totalBusy, count float64
				for _, engine := range engines {
					if !engine.IsDir() {
						continue
					}
					busyPath := filepath.Join(enginesPath, engine.Name(), "busy")
					if data, err := os.ReadFile(busyPath); err == nil {
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
		}
	}
	
	// Method 1b: Try alternative engine paths (some kernels use different structure)
	// Check for engines directly under device
	altEnginesPath := filepath.Join(cardPath, "engines")
	if engines, err := os.ReadDir(altEnginesPath); err == nil {
		var totalBusy, count float64
		for _, engine := range engines {
			if !engine.IsDir() {
				continue
			}
			busyPath := filepath.Join(altEnginesPath, engine.Name(), "busy")
			if data, err := os.ReadFile(busyPath); err == nil {
				busyStr := strings.TrimSpace(string(data))
				if busy, err := strconv.ParseFloat(busyStr, 64); err == nil {
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
	// Set timeout context or use a short timeout
	output, err := cmd.Output()
	if err != nil {
		return 0.0
	}

	// Parse output - intel_gpu_top shows engine utilization
	// Example output format:
	//   IMC read:     0.00 MiB/s
	//   IMC write:    0.00 MiB/s
	//   RC6:          99.99%
	//   Render/3D:     0.00%
	//   Blitter:       0.00%
	//   Video/0:      45.23%  <-- This is what we want
	//   Video/1:       0.00%
	lines := strings.Split(string(output), "\n")
	var totalUtil, count float64
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip RC6 (power state, not utilization)
		if strings.HasPrefix(line, "RC6:") {
			continue
		}
		
		// Look for engine utilization lines (Render, Video, Blitter, etc.)
		// Format: "Engine Name: XX.XX%"
		if strings.Contains(line, "%") && (strings.Contains(line, "Video") || 
			strings.Contains(line, "Render") || strings.Contains(line, "Blitter") ||
			strings.Contains(line, "Compute") || strings.Contains(line, "VCS") ||
			strings.Contains(line, "VECS")) {
			
			// Extract percentage - look for the last number with % sign
			parts := strings.Fields(line)
			for i := len(parts) - 1; i >= 0; i-- {
				part := parts[i]
				if strings.HasSuffix(part, "%") {
					percentStr := strings.TrimSuffix(part, "%")
					if util, err := strconv.ParseFloat(percentStr, 64); err == nil {
						// Only count non-zero, reasonable values
						if util > 0 && util <= 100 {
							totalUtil += util
							count++
						}
					}
					break
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

