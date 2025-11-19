package ffmpeg

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yourname/av1qsvd/internal/metadata"
)

// TranscodeArgs builds ffmpeg command arguments for AV1 QSV transcoding.
// Returns a slice of command-line arguments ready to be passed to exec.Command.
func TranscodeArgs(ffmpegPath, inputPath, outputPath string, probeResult *metadata.ProbeResult, isWebRipLike bool) ([]string, error) {
	if probeResult.VideoStream == nil {
		return nil, fmt.Errorf("no video stream found in probe result")
	}

	videoStream := probeResult.VideoStream
	videoIndex := videoStream.Index

	// Pure VAAPI approach for Intel Arc GPUs:
	// - Use VAAPI for both decoding and AV1 encoding
	// - Avoids QSV MFX session errors entirely
	// - Intel Arc GPUs support AV1 encoding via VAAPI directly
	log.Printf("Using pure VAAPI mode (decode + encode)")

	// Find render node for VAAPI
	renderNode := findRenderNode()
	if renderNode == "" {
		renderNode = "/dev/dri/renderD128" // fallback
	}

	// Build command arguments
	// Use VAAPI for everything - no QSV needed
	// Try auto-detection first, then fall back to explicit device
	args := []string{
		"-hide_banner",
		"-analyzeduration", "50M",
		"-probesize", "50M",
	}
	
	// Try VAAPI initialization - use auto-detection first (vaapi=va)
	// This lets VAAPI find the device automatically, which is more reliable
	args = append(args, "-init_hw_device", "vaapi=va")
	args = append(args,
		"-hwaccel", "vaapi",
		"-hwaccel_output_format", "vaapi",
		"-filter_hw_device", "va",  // Use VAAPI for filters
	)
	
	// Don't specify hwaccel_device - let VAAPI auto-detect
	// Explicit device paths can cause "No VA display found" errors

	// WebRip-specific input flags
	if isWebRipLike {
		args = append(args,
			"-fflags", "+genpts",
			"-copyts",
			"-start_at_zero",
		)
	}

	// Input file
	args = append(args, "-i", inputPath)

	// Stream mapping: start with all streams, then prune
	args = append(args,
		"-map", "0",
		"-map", "-0:v", // remove all video
		"-map", "-0:t", // remove attachments
		"-map", fmt.Sprintf("0:v:%d", videoIndex), // add only main video
		"-map", "0:a?", // all audio
		"-map", "-0:a:m:language:rus", // remove Russian audio
		"-map", "-0:a:m:language:ru", // remove Russian audio (alternate code)
		"-map", "0:s?", // all subtitles
		"-map", "-0:s:m:language:rus", // remove Russian subtitles
		"-map", "-0:s:m:language:ru", // remove Russian subtitles (alternate code)
		"-map_chapters", "0",
	)

	// Determine quality based on height
	quality := determineQuality(videoStream.Height)

	// Video filter chain
	// VAAPI decode outputs in vaapi format (hardware surfaces)
	// Try to use VAAPI-native filters where possible to avoid format conversion issues
	// For setsar, we need to download/upload, but we'll keep it simple
	var vfParts []string
	if isWebRipLike {
		// WebRip: scale to handle SAR using VAAPI scaling, then ensure even dimensions
		// Use scale_vaapi for hardware-accelerated scaling, then download/upload for setsar
		vfParts = append(vfParts,
			"scale_vaapi=w='if(gt(iw,iw*sar),iw,iw*sar)':h='if(gt(iw,iw*sar),iw/sar,ih)'",
			"scale_vaapi=w=ceil(iw/2)*2:h=ceil(ih/2)*2",
			"hwdownload,format=nv12",
			"setsar=1",
			"format=nv12",
			"hwupload",
		)
	} else {
		// Non-WebRip: ensure even dimensions using VAAPI, then set SAR
		vfParts = append(vfParts,
			"scale_vaapi=w=ceil(iw/2)*2:h=ceil(ih/2)*2",
			"hwdownload,format=nv12",
			"setsar=1",
			"format=nv12",
			"hwupload",
		)
	}

	args = append(args, "-vf:v:0", fmt.Sprintf("%s", joinFilterParts(vfParts)))

	// Video codec and encoding parameters
	// Use av1_vaapi encoder (Intel Arc GPUs support AV1 via VAAPI)
	args = append(args,
		"-c:v:0", "av1_vaapi",
		"-global_quality:v:0", fmt.Sprintf("%d", quality),
		"-compression_level", "2",  // VAAPI equivalent of preset
	)

	// WebRip-specific output flags
	if isWebRipLike {
		args = append(args,
			"-vsync", "0",
			"-avoid_negative_ts", "make_zero",
		)
	}

	// Audio and subtitle passthrough
	args = append(args,
		"-c:a", "copy",
		"-c:s", "copy",
	)

	// Container/muxing settings
	args = append(args,
		"-max_muxing_queue_size", "2048",
		"-map_metadata", "0",
		"-f", "matroska",
		"-movflags", "+faststart",
	)

	// Output file
	args = append(args, outputPath)

	return args, nil
}

// DetermineQuality returns the global_quality value based on video height.
// height >= 1440 → 23
// height >= 1080 && < 1440 → 24
// < 1080 → 25
func DetermineQuality(height int) int {
	if height >= 1440 {
		return 23
	}
	if height >= 1080 {
		return 24
	}
	return 25
}

// determineQuality is kept for internal use (backward compatibility)
func determineQuality(height int) int {
	return DetermineQuality(height)
}

// determineSurfaceFormat returns the surface format based on bit depth.
// bit depth >= 10 → p010, else → nv12
func determineSurfaceFormat(bitDepth int) string {
	if bitDepth >= 10 {
		return "p010"
	}
	return "nv12"
}

// joinFilterParts joins video filter parts with commas.
func joinFilterParts(parts []string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += ","
		}
		result += part
	}
	return result
}

// RunTranscode executes the ffmpeg transcode command and returns the exit code and any error.
func RunTranscode(ffmpegPath string, args []string) (int, error) {
	cmd := exec.Command(ffmpegPath, args...)
	// Set LD_LIBRARY_PATH to help static ffmpeg find dynamic VA-API libraries
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH=/lib/x86_64-linux-gnu:/usr/lib/x86_64-linux-gnu:"+os.Getenv("LD_LIBRARY_PATH"))

	// Capture both stdout and stderr for logging
	// Use stderr for better error visibility (ffmpeg outputs errors to stderr)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()

	if err != nil {
		// Try to extract exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			errOutput := stderr.String()
			if errOutput == "" {
				errOutput = string(output)
			}
			
			// Log full error output for debugging (last 5000 chars)
			fullError := errOutput
			if len(fullError) > 5000 {
				fullError = "... " + fullError[len(fullError)-5000:]
			}
			log.Printf("ffmpeg stderr output (last 5000 chars):\n%s", fullError)
			
			// Extract relevant error lines - look for error patterns
			lines := strings.Split(errOutput, "\n")
			var errorLines []string
			
			// Look for error patterns in the output
			errorKeywords := []string{
				"error", "Error", "ERROR",
				"failed", "Failed", "FAILED",
				"invalid", "Invalid", "INVALID",
				"cannot", "Cannot", "CANNOT",
				"unable", "Unable", "UNABLE",
				"not found", "No such",
				"Permission denied",
				"VAAPI", "vaapi",
				"encoder", "decoder",
			}
			
			// Collect error lines (last 50 lines, filtering for relevant ones)
			startLine := len(lines) - 50
			if startLine < 0 {
				startLine = 0
			}
			for i := startLine; i < len(lines); i++ {
				line := strings.TrimSpace(lines[i])
				if line == "" {
					continue
				}
				// Skip progress lines
				if strings.Contains(line, "frame=") || strings.Contains(line, "fps=") || 
				   strings.Contains(line, "bitrate=") || strings.Contains(line, "speed=") ||
				   strings.Contains(line, "time=") {
					continue
				}
				// Check if line contains error keywords
				for _, keyword := range errorKeywords {
					if strings.Contains(line, keyword) {
						errorLines = append(errorLines, line)
						break
					}
				}
			}
			
			// If we found error lines, use them; otherwise use last non-progress lines
			var relevantError string
			if len(errorLines) > 0 {
				// Take last 5 error lines
				start := len(errorLines) - 5
				if start < 0 {
					start = 0
				}
				relevantError = strings.Join(errorLines[start:], " | ")
			} else {
				// Fallback: get last few non-progress lines
				for i := len(lines) - 1; i >= 0 && i >= len(lines)-20; i-- {
					line := strings.TrimSpace(lines[i])
					if line != "" && !strings.Contains(line, "frame=") && 
					   !strings.Contains(line, "fps=") && !strings.Contains(line, "bitrate=") {
						if relevantError == "" {
							relevantError = line
						} else {
							relevantError = line + " | " + relevantError
						}
						if len(strings.Split(relevantError, " | ")) >= 3 {
							break
						}
					}
				}
			}
			
			// If still no relevant error, use last part of stderr
			if relevantError == "" {
				relevantError = fullError
			}
			
			// Limit the relevant error to 800 chars for the reason field
			if len(relevantError) > 800 {
				relevantError = relevantError[:800] + "..."
			}
			
			return exitError.ExitCode(), fmt.Errorf("ffmpeg failed with exit code %d: %s", exitError.ExitCode(), relevantError)
		}
		errOutput := stderr.String()
		if errOutput == "" {
			errOutput = string(output)
		}
		// Log full error for non-exit errors too
		if len(errOutput) > 5000 {
			log.Printf("ffmpeg execution error (last 5000 chars):\n%s", errOutput[len(errOutput)-5000:])
		} else {
			log.Printf("ffmpeg execution error:\n%s", errOutput)
		}
		return -1, fmt.Errorf("ffmpeg execution failed: %w: %s", err, errOutput)
	}

	return 0, nil
}

// findRenderNode finds the best DRI render node for VAAPI/QSV operations.
func findRenderNode() string {
	candidates := []string{
		"/dev/dri/renderD128",
		"/dev/dri/renderD129",
		"/dev/dri/renderD130",
	}
	
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	
	// If no specific render node found, try glob
	if matches, err := filepath.Glob("/dev/dri/renderD*"); err == nil && len(matches) > 0 {
		return matches[0]
	}
	
	return ""
}

// selectVAAPIQSVDevices picks the best VAAPI/QSV device initialization for Intel Arc GPUs.
// Returns VAAPI device init string, QSV device init string (derived from VAAPI), and filter device name.
// For Intel Arc GPUs, initializing QSV via VAAPI is more reliable than direct QSV initialization.
func selectVAAPIQSVDevices() (string, string, string) {
	renderNode := findRenderNode()
	
	// For Intel Arc GPUs, use VAAPI->QSV initialization pattern:
	// 1. Initialize VAAPI device first: vaapi=va:/dev/dri/renderD128
	// 2. Derive QSV from VAAPI: qsv=qsv@va
	// This is more reliable than direct QSV initialization
	if renderNode != "" {
		vaapiInit := fmt.Sprintf("vaapi=va:%s", renderNode)
		qsvInit := "qsv=qsv@va"  // Derive QSV from VAAPI device named "va"
		return vaapiInit, qsvInit, "qsv"
	}
	
	// Fallback: let ffmpeg auto-detect
	return "vaapi=va", "qsv=qsv@va", "qsv"
}
