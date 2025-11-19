package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	cardPath := "/sys/class/drm/card1/device"
	
	fmt.Printf("Original path: %s\n", cardPath)
	
	// Resolve symlink
	resolvedPath, err := filepath.EvalSymlinks(cardPath)
	if err != nil {
		fmt.Printf("EvalSymlinks failed: %v\n", err)
		// Try reading symlink manually
		if linkTarget, err := os.Readlink(cardPath); err == nil {
			fmt.Printf("Symlink target: %s\n", linkTarget)
			if !filepath.IsAbs(linkTarget) {
				resolvedPath = filepath.Join(filepath.Dir(cardPath), linkTarget)
				resolvedPath = filepath.Clean(resolvedPath)
			} else {
				resolvedPath = linkTarget
			}
		} else {
			fmt.Printf("Error reading symlink: %v\n", err)
			resolvedPath = cardPath
		}
	}
	fmt.Printf("Resolved path: %s\n", resolvedPath)
	
	// Try to find drm subdirectory
	drmPath := filepath.Join(resolvedPath, "drm")
	fmt.Printf("Looking for drm path: %s\n", drmPath)
	
	if _, err := os.Stat(drmPath); err != nil {
		fmt.Printf("drm path doesn't exist: %v\n", err)
		return
	}
	
	fmt.Printf("drm path exists!\n")
	
	// List card directories
	if entries, err := os.ReadDir(drmPath); err == nil {
		fmt.Printf("Found %d entries in drm:\n", len(entries))
		for _, entry := range entries {
			fmt.Printf("  - %s (dir: %v)\n", entry.Name(), entry.IsDir())
			
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "card") {
				continue
			}
			
			gtBase := filepath.Join(drmPath, entry.Name(), "gt")
			fmt.Printf("  Checking GT base: %s\n", gtBase)
			
			if gtDirs, err := os.ReadDir(gtBase); err == nil {
				for _, gtDir := range gtDirs {
					if !gtDir.IsDir() || !strings.HasPrefix(gtDir.Name(), "gt") {
						continue
					}
					
					fmt.Printf("    Found GT: %s\n", gtDir.Name())
					
					actFreqPath := filepath.Join(gtBase, gtDir.Name(), "rps_act_freq_mhz")
					maxFreqPath := filepath.Join(gtBase, gtDir.Name(), "rps_max_freq_mhz")
					
					fmt.Printf("      Act freq path: %s\n", actFreqPath)
					fmt.Printf("      Max freq path: %s\n", maxFreqPath)
					
					actFreqData, err1 := os.ReadFile(actFreqPath)
					maxFreqData, err2 := os.ReadFile(maxFreqPath)
					
					if err1 != nil {
						fmt.Printf("      Error reading act freq: %v\n", err1)
					}
					if err2 != nil {
						fmt.Printf("      Error reading max freq: %v\n", err2)
					}
					
					if err1 == nil && err2 == nil {
						actFreqStr := strings.TrimSpace(string(actFreqData))
						maxFreqStr := strings.TrimSpace(string(maxFreqData))
						
						fmt.Printf("      Act freq value: %s\n", actFreqStr)
						fmt.Printf("      Max freq value: %s\n", maxFreqStr)
						
						actFreq, err1 := strconv.ParseFloat(actFreqStr, 64)
						maxFreq, err2 := strconv.ParseFloat(maxFreqStr, 64)
						
						if err1 != nil {
							fmt.Printf("      Error parsing act freq: %v\n", err1)
						}
						if err2 != nil {
							fmt.Printf("      Error parsing max freq: %v\n", err2)
						}
						
						if err1 == nil && err2 == nil && maxFreq > 0 {
							usage := (actFreq / maxFreq) * 100.0
							fmt.Printf("      GPU Usage: %.2f%%\n", usage)
						}
					}
				}
			} else {
				fmt.Printf("  Error reading GT base: %v\n", err)
			}
		}
	}
}

