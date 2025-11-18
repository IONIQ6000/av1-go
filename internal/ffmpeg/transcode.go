package ffmpeg

import (
	"fmt"
	"os/exec"

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

	// Build command arguments
	args := []string{
		"-hide_banner",
		"-hwaccel", "none",
		"-init_hw_device", "qsv=hw",
		"-filter_hw_device", "hw",
		"-analyzeduration", "50M",
		"-probesize", "50M",
	}

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
		"-map", "-0:v",        // remove all video
		"-map", "-0:t",        // remove attachments
		"-map", fmt.Sprintf("0:v:%d", videoIndex), // add only main video
		"-map", "0:a?",        // all audio
		"-map", "-0:a:m:language:rus", // remove Russian audio
		"-map", "-0:a:m:language:ru",  // remove Russian audio (alternate code)
		"-map", "0:s?",        // all subtitles
		"-map", "-0:s:m:language:rus", // remove Russian subtitles
		"-map", "-0:s:m:language:ru",  // remove Russian subtitles (alternate code)
		"-map_chapters", "0",
	)

	// Determine quality based on height
	quality := determineQuality(videoStream.Height)

	// Determine surface format based on bit depth
	surfaceFormat := determineSurfaceFormat(videoStream.BitDepth)

	// Video filter chain
	var vfParts []string
	if isWebRipLike {
		// WebRip: pad to even dimensions, set SAR, format, hwupload
		vfParts = append(vfParts,
			"pad=ceil(iw/2)*2:ceil(ih/2)*2",
			"setsar=1",
			fmt.Sprintf("format=%s", surfaceFormat),
			"hwupload=extra_hw_frames=64",
		)
	} else {
		// Non-WebRip: still pad and format, but no timestamp flags
		vfParts = append(vfParts,
			"pad=ceil(iw/2)*2:ceil(ih/2)*2",
			"setsar=1",
			fmt.Sprintf("format=%s", surfaceFormat),
			"hwupload=extra_hw_frames=64",
		)
	}

	args = append(args, "-vf:v:0", fmt.Sprintf("%s", joinFilterParts(vfParts)))

	// Video codec and encoding parameters
	args = append(args,
		"-c:v:0", "av1_qsv",
		"-global_quality:v:0", fmt.Sprintf("%d", quality),
		"-preset:v:0", "medium",
		"-look_ahead", "1",
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

// determineQuality returns the global_quality value based on video height.
// height >= 1440 → 23
// height >= 1080 && < 1440 → 24
// < 1080 → 25
func determineQuality(height int) int {
	if height >= 1440 {
		return 23
	}
	if height >= 1080 {
		return 24
	}
	return 25
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
	
	// Capture both stdout and stderr for logging
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		// Try to extract exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			return exitError.ExitCode(), fmt.Errorf("ffmpeg failed with exit code %d: %s", exitError.ExitCode(), string(output))
		}
		return -1, fmt.Errorf("ffmpeg execution failed: %w: %s", err, string(output))
	}

	return 0, nil
}
