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
	testCmd := exec.Command(
		ffmpegPath,
		"-hide_banner",
		"-v", "error",
		"-init_hw_device", "qsv=hw",
		"-filter_hw_device", "hw",
		"-f", "lavfi",
		"-i", "testsrc2=s=1280x720:d=1",
		"-vf", "format=nv12,hwupload=extra_hw_frames=64",
		"-frames:v", "1",
		"-c:v", "av1_qsv",
		"-global_quality", "30",
		"-f", "null",
		"-",
	)

	testOutput, err := testCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("QSV test failed: %w (output: %s)", err, string(testOutput))
	}

	log.Printf("ffmpeg verification passed")
	return nil
}

