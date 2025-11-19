package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// getGPUUsage attempts to get Intel GPU utilization percentage.
// Returns 0.0 if unable to determine GPU usage.
func getGPUUsage() float64 {
	// Try to find Intel GPU card
	cardPath := findIntelGPUCard()
	if cardPath == "" {
		return 0.0
	}

	// Try multiple methods to get GPU utilization (in order of preference)
	
	// Method 1: Read from sysfs engine utilization (most reliable if available)
	if usage := getGPUUsageFromSysfs(cardPath); usage > 0 {
		return usage
	}

	// Method 2: Try intel_gpu_top (may require PMU permissions)
	// Only try if sysfs methods failed
	if usage := getGPUUsageFromIntelGPUTop(); usage > 0 {
		return usage
	}

	// If all methods fail, return 0.0
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
	// Method 1: Resolve symlink to actual PCI device path
	// cardPath is like /sys/class/drm/card1/device which is a symlink
	// The symlink points to something like ../../../0000:03:00.0
	// We need to resolve it to the actual PCI device path
	resolvedPath, err := filepath.EvalSymlinks(cardPath)
	if err != nil {
		// If symlink resolution fails, try reading the symlink manually
		if linkTarget, err := os.Readlink(cardPath); err == nil {
			// Resolve relative symlink
			if !filepath.IsAbs(linkTarget) {
				resolvedPath = filepath.Join(filepath.Dir(cardPath), linkTarget)
				// Clean the path to resolve ../
				resolvedPath = filepath.Clean(resolvedPath)
			} else {
				resolvedPath = linkTarget
			}
		} else {
			// If all else fails, try direct path
			resolvedPath = cardPath
		}
	}
	
	// For Intel Arc GPUs, the GT directory is under drm/cardX/gt/gt0
	// Path structure: /sys/devices/pci.../drm/card1/gt/gt0/
	// The resolved path should be the PCI device, and drm/ is a subdirectory
	drmPath := filepath.Join(resolvedPath, "drm")
	if _, err := os.Stat(drmPath); err == nil {
		// Look for card directories under drm
		if entries, err := os.ReadDir(drmPath); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "card") {
					continue
				}
				
				// Try GT paths under this card
				gtBase := filepath.Join(drmPath, entry.Name(), "gt")
				if gtDirs, err := os.ReadDir(gtBase); err == nil {
					for _, gtDir := range gtDirs {
						if !gtDir.IsDir() || !strings.HasPrefix(gtDir.Name(), "gt") {
							continue
						}
						
						// Try to read frequency files
						actFreqPath := filepath.Join(gtBase, gtDir.Name(), "rps_act_freq_mhz")
						maxFreqPath := filepath.Join(gtBase, gtDir.Name(), "rps_max_freq_mhz")
						
						// Also try alternative names
						if _, err := os.Stat(actFreqPath); err != nil {
							actFreqPath = filepath.Join(gtBase, gtDir.Name(), "rps_cur_freq_mhz")
						}
						
						actFreqData, err1 := os.ReadFile(actFreqPath)
						maxFreqData, err2 := os.ReadFile(maxFreqPath)
						
						if err1 == nil && err2 == nil {
							actFreqStr := strings.TrimSpace(string(actFreqData))
							maxFreqStr := strings.TrimSpace(string(maxFreqData))
							
							actFreq, err1 := strconv.ParseFloat(actFreqStr, 64)
							maxFreq, err2 := strconv.ParseFloat(maxFreqStr, 64)
							
							if err1 == nil && err2 == nil && maxFreq > 0 {
								usage := (actFreq / maxFreq) * 100.0
								if usage > 100.0 {
									usage = 100.0
								}
								if usage > 0 {
									return usage
								}
							}
						}
						
						// Try engines directory if it exists
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
			}
		}
	}

	// Method 2: Try searching for frequency files directly
	// Sometimes the path structure is different, so search for the files
	searchPaths := []string{
		"/sys/devices/pci0000:00/0000:00:01.1/0000:01:00.0/0000:02:01.0/0000:03:00.0/drm/card1/gt/gt0",
		"/sys/devices/pci0000:00/0000:00:01.0/0000:01:00.0/drm/card1/gt/gt0",
		filepath.Join(resolvedPath, "drm", "card1", "gt", "gt0"),
		filepath.Join(resolvedPath, "drm", "card0", "gt", "gt0"),
	}
	
	for _, searchPath := range searchPaths {
		actFreqPath := filepath.Join(searchPath, "rps_act_freq_mhz")
		maxFreqPath := filepath.Join(searchPath, "rps_max_freq_mhz")
		
		actFreqData, err1 := os.ReadFile(actFreqPath)
		maxFreqData, err2 := os.ReadFile(maxFreqPath)
		
		if err1 == nil && err2 == nil {
			actFreqStr := strings.TrimSpace(string(actFreqData))
			maxFreqStr := strings.TrimSpace(string(maxFreqData))
			
			actFreq, err1 := strconv.ParseFloat(actFreqStr, 64)
			maxFreq, err2 := strconv.ParseFloat(maxFreqStr, 64)
			
			if err1 == nil && err2 == nil && maxFreq > 0 {
				usage := (actFreq / maxFreq) * 100.0
				if usage > 100.0 {
					usage = 100.0
				}
				if usage >= 0 {
					return usage
				}
			}
		}
		
		// Also try rps_cur_freq_mhz as alternative
		curFreqPath := filepath.Join(searchPath, "rps_cur_freq_mhz")
		if curFreqData, err := os.ReadFile(curFreqPath); err == nil {
			if maxFreqData, err2 := os.ReadFile(maxFreqPath); err2 == nil {
				curFreqStr := strings.TrimSpace(string(curFreqData))
				maxFreqStr := strings.TrimSpace(string(maxFreqData))
				
				curFreq, err1 := strconv.ParseFloat(curFreqStr, 64)
				maxFreq, err2 := strconv.ParseFloat(maxFreqStr, 64)
				
				if err1 == nil && err2 == nil && maxFreq > 0 {
					usage := (curFreq / maxFreq) * 100.0
					if usage > 100.0 {
						usage = 100.0
					}
					if usage >= 0 {
						return usage
					}
				}
			}
		}
	}

	// Method 3: Try direct GT path (fallback for older kernels)
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
	
	// Method 3: Try alternative engine paths (some kernels use different structure)
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

	// Method 2: Try reading from /sys/kernel/debug/dri (if debugfs is mounted)
	// This requires debugfs to be mounted: mount -t debugfs none /sys/kernel/debug
	debugPaths := []string{
		"/sys/kernel/debug/dri/1/i915_engine_info",
		"/sys/kernel/debug/dri/0/i915_engine_info",
	}
	for _, debugPath := range debugPaths {
		if data, err := os.ReadFile(debugPath); err == nil {
			// Parse engine info - look for busy percentages
			// Format varies, but typically shows engine utilization
			lines := strings.Split(string(data), "\n")
			var totalBusy, count float64
			for _, line := range lines {
				line = strings.ToLower(line)
				// Look for busy or utilization patterns
				if strings.Contains(line, "busy") || strings.Contains(line, "util") {
					// Try to extract percentage
					fields := strings.Fields(line)
					for _, field := range fields {
						if strings.HasSuffix(field, "%") {
							percentStr := strings.TrimSuffix(field, "%")
							if busy, err := strconv.ParseFloat(percentStr, 64); err == nil {
								if busy > 0 && busy <= 100 {
									totalBusy += busy
									count++
								}
							}
						}
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

	// Method 3: Read from intel_gpu_frequency (if available)
	// Try multiple GT paths
	for gtNum := 0; gtNum < 4; gtNum++ {
		freqPath := filepath.Join(cardPath, "gt", fmt.Sprintf("gt%d", gtNum), "intel_gpu_freq")
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
						maxFreqPath := filepath.Join(cardPath, "gt", fmt.Sprintf("gt%d", gtNum), "rps_max_freq_mhz")
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
	// Try to run intel_gpu_top with -l flag (plain text output)
	// Note: -l outputs continuously, so we use context timeout
	// PMU access may require root or special permissions
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "intel_gpu_top", "-l", "-s", "500")
	output, err := cmd.CombinedOutput()
	
	// Check if PMU permission error
	outputStr := string(output)
	if strings.Contains(outputStr, "PMU") && strings.Contains(outputStr, "Permission denied") {
		// PMU access denied - try with sudo if available, or skip
		// For now, return 0 to fall back to other methods
		return 0.0
	}
	
	if err != nil {
		// Context timeout is expected - we got the output we need
		if ctx.Err() == context.DeadlineExceeded {
			// This is fine, we got output before timeout
		} else {
			return 0.0
		}
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

