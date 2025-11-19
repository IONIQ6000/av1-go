package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Copy of readFreqFiles from gpu.go
func readFreqFiles(gtPath string) float64 {
	actFreqPath := filepath.Join(gtPath, "rps_act_freq_mhz")
	maxFreqPath := filepath.Join(gtPath, "rps_max_freq_mhz")
	
	fmt.Printf("Attempting to read: %s\n", actFreqPath)
	fmt.Printf("Attempting to read: %s\n", maxFreqPath)
	
	actFreqData, err1 := os.ReadFile(actFreqPath)
	maxFreqData, err2 := os.ReadFile(maxFreqPath)
	
	fmt.Printf("Read errors: err1=%v, err2=%v\n", err1, err2)
	
	if err1 == nil && err2 == nil {
		actFreqStr := strings.TrimSpace(string(actFreqData))
		maxFreqStr := strings.TrimSpace(string(maxFreqData))
		
		fmt.Printf("Raw values: actFreqStr=%q, maxFreqStr=%q\n", actFreqStr, maxFreqStr)
		
		actFreq, err1 := strconv.ParseFloat(actFreqStr, 64)
		maxFreq, err2 := strconv.ParseFloat(maxFreqStr, 64)
		
		fmt.Printf("Parsed values: actFreq=%.2f, maxFreq=%.2f, parse errors: err1=%v, err2=%v\n", actFreq, maxFreq, err1, err2)
		
		if err1 == nil && err2 == nil && maxFreq > 0 {
			usage := (actFreq / maxFreq) * 100.0
			if usage > 100.0 {
				usage = 100.0
			}
			fmt.Printf("Calculated usage: %.2f%%\n", usage)
			return usage
		}
	}
	
	fmt.Printf("Failed to read/parse, returning 0.0\n")
	return 0.0
}

// Copy of getGPUUsage logic
func getGPUUsage() float64 {
	hardcodedPath := "/sys/devices/pci0000:00/0000:00:01.1/0000:01:00.0/0000:02:01.0/0000:03:00.0/drm/card1/gt/gt0"
	actFreqPath := filepath.Join(hardcodedPath, "rps_act_freq_mhz")
	maxFreqPath := filepath.Join(hardcodedPath, "rps_max_freq_mhz")
	
	fmt.Printf("\n=== getGPUUsage() called ===\n")
	fmt.Printf("Checking if files exist:\n")
	fmt.Printf("  %s\n", actFreqPath)
	fmt.Printf("  %s\n", maxFreqPath)
	
	stat1, err1 := os.Stat(actFreqPath)
	stat2, err2 := os.Stat(maxFreqPath)
	
	fmt.Printf("Stat results: err1=%v, err2=%v\n", err1, err2)
	if stat1 != nil {
		fmt.Printf("  File 1 exists: %v, size: %d\n", stat1 != nil, stat1.Size())
	}
	if stat2 != nil {
		fmt.Printf("  File 2 exists: %v, size: %d\n", stat2 != nil, stat2.Size())
	}
	
	if err1 == nil && err2 == nil {
		fmt.Printf("Both files exist, calling readFreqFiles()...\n")
		usage := readFreqFiles(hardcodedPath)
		fmt.Printf("readFreqFiles returned: %.2f%%\n", usage)
		return usage
	}
	
	fmt.Printf("Files don't exist or error checking, returning 0.0\n")
	return 0.0
}

func main() {
	fmt.Println("Testing getGPUUsage() logic...")
	usage := getGPUUsage()
	fmt.Printf("\nFinal result: %.2f%%\n", usage)
}

