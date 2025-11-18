package scan

import (
	"fmt"
	"os"
	"time"
)

// CheckFileStable checks if a file's size is stable (not currently being copied).
// It checks the size at t0, waits for waitSeconds, then checks again.
// If the size changed, returns false (file is still copying).
// If size is stable, returns true.
func CheckFileStable(filePath string, waitSeconds int) (bool, error) {
	// Get initial size
	info0, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to stat file: %w", err)
	}
	size0 := info0.Size()

	// Wait
	time.Sleep(time.Duration(waitSeconds) * time.Second)

	// Get size again
	info1, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to stat file after wait: %w", err)
	}
	size1 := info1.Size()

	// File is stable if sizes match
	return size0 == size1, nil
}
