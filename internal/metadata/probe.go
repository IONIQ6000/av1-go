package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProbeResult contains the parsed ffprobe output for a media file.
type ProbeResult struct {
	Format      FormatInfo   `json:"format"`
	Streams     []StreamInfo `json:"streams"`
	HasVideo    bool
	HasAV1      bool
	IsWebRipLike bool
	VideoStream *StreamInfo // Main video stream (first or default-disposition)
}

// FormatInfo contains format-level metadata from ffprobe.
type FormatInfo struct {
	FormatName string `json:"format_name"`
	Duration   string `json:"duration"`
	Size       string `json:"size"`
}

// StreamInfo contains stream-level metadata from ffprobe.
type StreamInfo struct {
	Index          int    `json:"index"`
	CodecName      string `json:"codec_name"`
	CodecType      string `json:"codec_type"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	AvgFrameRate   string `json:"avg_frame_rate"`
	RFrameRate     string `json:"r_frame_rate"`
	BitDepth       int    `json:"bits_per_raw_sample,omitempty"`
	Disposition    map[string]int `json:"disposition"`
}

// ProbeFile runs ffprobe on a file and returns parsed metadata.
// Uses ffmpeg binary (which includes ffprobe functionality) at the given path.
func ProbeFile(ffmpegPath, filePath string) (*ProbeResult, error) {
	// Validate ffmpegPath is not empty
	if ffmpegPath == "" {
		return nil, fmt.Errorf("ffprobe failed: ffmpeg path is empty")
	}
	
	// Use ffmpeg to probe (ffmpeg includes ffprobe functionality)
	cmd := exec.Command(
		ffmpegPath,
		"-hide_banner",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var result ProbeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe JSON: %w", err)
	}

	// Analyze streams
	result.HasVideo = false
	result.HasAV1 = false
	var videoStreams []StreamInfo

	for i := range result.Streams {
		stream := &result.Streams[i]
		if stream.CodecType == "video" {
			result.HasVideo = true
			videoStreams = append(videoStreams, *stream)

			// Check if codec is AV1
			if stream.CodecName == "av1" {
				result.HasAV1 = true
			}
		}
	}

	// Select main video stream (prefer default disposition, else first)
	if len(videoStreams) > 0 {
		for i := range videoStreams {
			if videoStreams[i].Disposition != nil && videoStreams[i].Disposition["default"] == 1 {
				result.VideoStream = &videoStreams[i]
				break
			}
		}
		if result.VideoStream == nil {
			result.VideoStream = &videoStreams[0]
		}
	}

	// Determine if WebRip-like
	result.IsWebRipLike = isWebRipLike(&result)

	return &result, nil
}

// isWebRipLike determines if a file is WebRip-like based on heuristics.
// A file is WebRip-like if ANY of:
// - format_name contains "mp4", "mov", or "webm"
// - Any video stream has avg_frame_rate != r_frame_rate (VFR)
// - Any video stream has odd dimensions (width or height not divisible by 2)
func isWebRipLike(result *ProbeResult) bool {
	// Check format name
	formatName := strings.ToLower(result.Format.FormatName)
	if strings.Contains(formatName, "mp4") || strings.Contains(formatName, "mov") || strings.Contains(formatName, "webm") {
		return true
	}

	// Check video streams
	for _, stream := range result.Streams {
		if stream.CodecType != "video" {
			continue
		}

		// Check for VFR (variable frame rate)
		if stream.AvgFrameRate != "" && stream.RFrameRate != "" {
			if stream.AvgFrameRate != stream.RFrameRate {
				return true
			}
		}

		// Check for odd dimensions
		if stream.Width > 0 && stream.Width%2 != 0 {
			return true
		}
		if stream.Height > 0 && stream.Height%2 != 0 {
			return true
		}
	}

	return false
}

// WriteWhyFile writes a .why.txt sidecar file explaining why a file was skipped or rejected.
func WriteWhyFile(filePath, reason string) error {
	basePath := strings.TrimSuffix(filePath, filepath.Ext(filePath))
	whyPath := basePath + ".why.txt"
	return os.WriteFile(whyPath, []byte(reason), 0644)
}
