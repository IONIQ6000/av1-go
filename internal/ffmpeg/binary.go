package ffmpeg

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

// EnsureFFmpeg ensures that ffmpeg is installed and verified at the specified install directory.
// It downloads and extracts ffmpeg if it doesn't exist, then verifies it.
// Returns the path to the ffmpeg binary, or an error if installation or verification fails.
func EnsureFFmpeg(installDir, ffmpegURL string) (string, error) {
	ffmpegPath := filepath.Join(installDir, "ffmpeg")

	// Check if ffmpeg already exists and is executable
	if info, err := os.Stat(ffmpegPath); err == nil {
		if info.Mode().Perm()&0111 != 0 {
			log.Printf("ffmpeg found at %s", ffmpegPath)
			// Verify it anyway to ensure it's working
			if err := VerifyFFmpeg(ffmpegPath); err != nil {
				// Check if it's a library issue - don't re-download if libraries are missing
				errStr := err.Error()
				if strings.Contains(errStr, "missing VA-API libraries") || strings.Contains(errStr, "libva-drm.so") {
					log.Printf("ffmpeg verification failed due to missing libraries: %v", err)
					return "", err // Return the error so user can install libraries
				}
				log.Printf("Existing ffmpeg failed verification: %v", err)
				log.Printf("Re-downloading ffmpeg...")
				// Remove the broken binary and re-download
				if err := os.Remove(ffmpegPath); err != nil {
					return "", fmt.Errorf("failed to remove broken ffmpeg: %w", err)
				}
			} else {
				return ffmpegPath, nil
			}
		}
	}

	// ffmpeg doesn't exist or failed verification, download it
	log.Printf("Downloading ffmpeg from %s...", ffmpegURL)
	if err := downloadAndExtractFFmpeg(installDir, ffmpegURL); err != nil {
		return "", fmt.Errorf("failed to download/extract ffmpeg: %w", err)
	}

	// Verify the newly installed ffmpeg
	if err := VerifyFFmpeg(ffmpegPath); err != nil {
		return "", fmt.Errorf("ffmpeg verification failed: %w", err)
	}

	log.Printf("ffmpeg successfully installed and verified at %s", ffmpegPath)
	return ffmpegPath, nil
}

// downloadAndExtractFFmpeg downloads the ffmpeg archive, decompresses it, and extracts the binary.
func downloadAndExtractFFmpeg(installDir, url string) error {
	// Create install directory if it doesn't exist
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Download the archive
	log.Printf("Downloading %s...", url)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	// Read the entire response into memory (archive is compressed, so reasonable size)
	archiveData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read archive data: %w", err)
	}

	// Decompress .xz
	log.Printf("Decompressing .xz archive...")
	xzReader, err := xz.NewReader(bytes.NewReader(archiveData))
	if err != nil {
		return fmt.Errorf("failed to create xz reader: %w", err)
	}

	// Extract tar archive
	log.Printf("Extracting tar archive...")
	tarReader := tar.NewReader(xzReader)

	var ffmpegBinary []byte
	var foundFFmpeg bool

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Look for the ffmpeg binary
		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == "ffmpeg" {
			log.Printf("Found ffmpeg binary in archive at %s", header.Name)
			ffmpegBinary, err = io.ReadAll(tarReader)
			if err != nil {
				return fmt.Errorf("failed to read ffmpeg binary from archive: %w", err)
			}
			foundFFmpeg = true
			break
		}
	}

	if !foundFFmpeg {
		return fmt.Errorf("ffmpeg binary not found in archive")
	}

	// Write the binary to the install directory
	ffmpegPath := filepath.Join(installDir, "ffmpeg")
	if err := os.WriteFile(ffmpegPath, ffmpegBinary, 0755); err != nil {
		return fmt.Errorf("failed to write ffmpeg binary: %w", err)
	}

	log.Printf("ffmpeg binary extracted to %s", ffmpegPath)
	return nil
}

// VerifyFFmpeg verifies that the ffmpeg binary is working correctly.
// It checks:
// 1. Version string starts with "ffmpeg version 8." or "ffmpeg version n8.0"
// 2. av1_qsv encoder is available
// 3. QSV hardware acceleration test passes
func VerifyFFmpeg(ffmpegPath string) error {
	// Check version
	log.Printf("Verifying ffmpeg version...")
	versionCmd := exec.Command(ffmpegPath, "-version")
	versionOutput, err := versionCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run ffmpeg -version: %w", err)
	}

	versionStr := string(versionOutput)
	if !strings.HasPrefix(versionStr, "ffmpeg version 8.") && !strings.HasPrefix(versionStr, "ffmpeg version n8.0") {
		return fmt.Errorf("unexpected ffmpeg version: %s", strings.Split(versionStr, "\n")[0])
	}

	// Check for av1_qsv encoder
	log.Printf("Checking for av1_qsv encoder...")
	encodersCmd := exec.Command(ffmpegPath, "-hide_banner", "-encoders")
	encodersOutput, err := encodersCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run ffmpeg -encoders: %w", err)
	}

	if !strings.Contains(string(encodersOutput), "av1_qsv") {
		return fmt.Errorf("av1_qsv encoder not found in ffmpeg build")
	}

	// Run QSV test
	log.Printf("Running QSV hardware acceleration test...")
	
	// First check if GPU devices are accessible
	driDevices := []string{"/dev/dri/renderD128", "/dev/dri/card0", "/dev/dri/renderD129"}
	hasGPUDevice := false
	for _, device := range driDevices {
		if _, err := os.Stat(device); err == nil {
			log.Printf("Found GPU device: %s", device)
			hasGPUDevice = true
		}
	}
	if !hasGPUDevice {
		log.Printf("Warning: No GPU devices found in /dev/dri/")
	}
	
	// Try different QSV initialization methods
	// Format: -init_hw_device qsv=<name>[:<device>]
	// Then use that name in -filter_hw_device
	testMethods := []struct {
		initDevice   string
		filterDevice string
		description  string
	}{
		{"qsv=qsv", "qsv", "QSV device (default)"},
		{"qsv=qsv:/dev/dri/renderD128", "qsv", "QSV with renderD128"},
		{"qsv=qsv:/dev/dri/card0", "qsv", "QSV with card0"},
	}
	
	var lastErr error
	var lastOutput string
	
	for _, method := range testMethods {
		log.Printf("Trying QSV device: %s (init: %s, filter: %s)", method.description, method.initDevice, method.filterDevice)
		
		// Build command with explicit arguments
		args := []string{
			"-hide_banner",
			"-v", "error",
			"-init_hw_device", method.initDevice,
			"-filter_hw_device", method.filterDevice,
			"-f", "lavfi",
			"-i", "testsrc2=s=1280x720:d=1",
			"-vf", "format=nv12,hwupload=extra_hw_frames=64",
			"-frames:v", "1",
			"-c:v", "av1_qsv",
			"-global_quality", "30",
			"-f", "null",
			"-",
		}
		
		testCmd := exec.Command(ffmpegPath, args...)
		testOutput, err := testCmd.CombinedOutput()
		if err == nil {
			log.Printf("QSV test passed with device: %s", method.description)
			return nil
		}
		
		lastErr = err
		lastOutput = string(testOutput)
		log.Printf("QSV test failed with %s: %v", method.description, err)
		if len(lastOutput) > 0 {
			log.Printf("  Output: %s", strings.TrimSpace(lastOutput))
		}
	}
	
	// All methods failed
	outputStr := lastOutput
	// Check for common library missing errors
	if strings.Contains(outputStr, "libva-drm.so") || strings.Contains(outputStr, "cannot open shared object file") {
		return fmt.Errorf("QSV test failed: missing VA-API libraries. Install with: sudo apt-get install libva-drm2 libva2 intel-media-va-driver-non-free libdrm-intel1. Error: %w (output: %s)", lastErr, outputStr)
	}
	// Check for device access errors
	if strings.Contains(outputStr, "Device creation failed") || strings.Contains(outputStr, "Generic error in an external library") {
		return fmt.Errorf("QSV test failed: GPU device not accessible. Check: 1) GPU is available (run 'vainfo'), 2) Service user has access to /dev/dri/*, 3) Intel GPU drivers are installed. Error: %w (output: %s)", lastErr, outputStr)
	}
	// Check for invalid device specification (might indicate QSV not properly configured)
	if strings.Contains(outputStr, "Invalid device specification") || strings.Contains(outputStr, "unknown device type") {
		return fmt.Errorf("QSV test failed: Invalid device specification. This may indicate: 1) QSV not properly compiled in ffmpeg, 2) GPU drivers not installed, 3) Try running 'vainfo' to verify GPU access. Error: %w (output: %s)", lastErr, outputStr)
	}
	return fmt.Errorf("QSV test failed: %w (output: %s)", lastErr, outputStr)
}

