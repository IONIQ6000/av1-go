package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ProbeResult contains the parsed ffprobe output for a media file.
type ProbeResult struct {
	Format       FormatInfo   `json:"format"`
	Streams      []StreamInfo `json:"streams"`
	HasVideo     bool
	HasAV1       bool
	IsWebRipLike bool         // Deprecated: use SourceDecision instead
	SourceDecision *WebSourceDecision // New scored classifier decision
	VideoStream  *StreamInfo // Main video stream (first or default-disposition)
}

// FormatInfo contains format-level metadata from ffprobe.
type FormatInfo struct {
	FormatName    string            `json:"format_name"`
	Duration      string            `json:"duration"`
	Size          string            `json:"size"`
	BitRate       string            `json:"bit_rate,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"` // muxing_app, writing_library, encoder, etc.
}

// StreamInfo contains stream-level metadata from ffprobe.
type StreamInfo struct {
	Index        int            `json:"index"`
	CodecName    string         `json:"codec_name"`
	CodecType    string         `json:"codec_type"`
	Width        int            `json:"width"`
	Height       int            `json:"height"`
	AvgFrameRate string         `json:"avg_frame_rate"`
	RFrameRate   string         `json:"r_frame_rate"`
	BitDepth     FlexibleInt    `json:"bits_per_raw_sample,omitempty"`
	BitRate      string         `json:"bit_rate,omitempty"`
	Disposition  map[string]int `json:"disposition"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// FlexibleInt is a helper type that can unmarshal ints represented as numbers or strings.
type FlexibleInt int

// UnmarshalJSON allows FlexibleInt to parse numeric JSON values that may be strings or numbers.
func (fi *FlexibleInt) UnmarshalJSON(data []byte) error {
	// Handle null
	if string(data) == "null" {
		*fi = 0
		return nil
	}

	// Try as integer
	var intVal int
	if err := json.Unmarshal(data, &intVal); err == nil {
		*fi = FlexibleInt(intVal)
		return nil
	}

	// Try as string
	var strVal string
	if err := json.Unmarshal(data, &strVal); err == nil {
		if strVal == "" {
			*fi = 0
			return nil
		}
		parsed, err := strconv.Atoi(strVal)
		if err != nil {
			return fmt.Errorf("invalid FlexibleInt value %q: %w", strVal, err)
		}
		*fi = FlexibleInt(parsed)
		return nil
	}

	return fmt.Errorf("invalid FlexibleInt JSON: %s", string(data))
}

// SourceClass represents the classification of a video source.
type SourceClass int

const (
	SourceUnknown SourceClass = iota
	SourceDiscLike
	SourceWebLike
)

// String returns a human-readable representation of SourceClass.
func (sc SourceClass) String() string {
	switch sc {
	case SourceDiscLike:
		return "DiscLike"
	case SourceWebLike:
		return "WebLike"
	default:
		return "Unknown"
	}
}

// WebSourceDecision represents a classification decision with score and reasons.
type WebSourceDecision struct {
	Class   SourceClass
	Score   float64
	Reasons []string
}

// IsWebLike returns true if the decision indicates web-like content.
// Unknown is treated conservatively as web-like (to apply web-safe flags).
func (d *WebSourceDecision) IsWebLike() bool {
	return d.Class == SourceWebLike || d.Class == SourceUnknown
}

// String returns a human-readable summary of the decision.
func (d *WebSourceDecision) String() string {
	return fmt.Sprintf("%s (score: %.1f, reasons: %s)", d.Class.String(), d.Score, strings.Join(d.Reasons, "; "))
}

// ProbeFile runs ffprobe on a file and returns parsed metadata.
// Uses ffprobe binary (or ffmpeg if ffprobe is not available) at the given path.
func ProbeFile(ffmpegPath, filePath string) (*ProbeResult, error) {
	// Store filePath for WebRip detection
	probeFilePath := filePath
	// Validate ffmpegPath is not empty
	if ffmpegPath == "" {
		return nil, fmt.Errorf("ffprobe failed: ffmpeg path is empty")
	}

	// Try to use ffprobe if available (it's in the same directory as ffmpeg)
	installDir := filepath.Dir(ffmpegPath)
	ffprobePath := filepath.Join(installDir, "ffprobe")

	// Check if ffprobe exists
	if _, err := os.Stat(ffprobePath); err != nil {
		// ffprobe not found, return error
		// ffmpeg doesn't support ffprobe flags, so we need ffprobe
		return nil, fmt.Errorf("ffprobe not found at %s (required for probing)", ffprobePath)
	}

	// Use ffprobe with proper flags
	cmd := exec.Command(
		ffprobePath,
		"-hide_banner",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		filePath,
	)
	// Set LD_LIBRARY_PATH to help static ffmpeg/ffprobe find dynamic libraries
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH=/lib/x86_64-linux-gnu:/usr/lib/x86_64-linux-gnu:"+os.Getenv("LD_LIBRARY_PATH"))

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

	// Classify source using scored classifier
	result.SourceDecision = ClassifyWebSource(probeFilePath, &result.Format, result.Streams)
	// Maintain backward compatibility
	result.IsWebRipLike = result.SourceDecision.IsWebLike()

	return &result, nil
}

// ClassifyWebSource classifies a video source as WebLike, DiscLike, or Unknown
// using a scored heuristic system with explainable reasons.
func ClassifyWebSource(filePath string, format *FormatInfo, streams []StreamInfo) *WebSourceDecision {
	decision := &WebSourceDecision{
		Class:   SourceUnknown,
		Score:   0.0,
		Reasons: []string{},
	}

	fileName := strings.ToLower(filepath.Base(filePath))
	dirName := strings.ToLower(filepath.Dir(filePath))
	ext := strings.ToLower(filepath.Ext(filePath))
	formatName := strings.ToLower(format.FormatName)

	// Check for explicit overrides via sidecar files
	basePath := strings.TrimSuffix(filePath, ext)
	if _, err := os.Stat(basePath + ".websafe"); err == nil {
		decision.Class = SourceWebLike
		decision.Score = 10.0 // Strong override
		decision.Reasons = []string{"override: .websafe sidecar file"}
		return decision
	}
	if _, err := os.Stat(basePath + ".nowebsafe"); err == nil {
		decision.Class = SourceDiscLike
		decision.Score = -10.0 // Strong override
		decision.Reasons = []string{"override: .nowebsafe sidecar file"}
		return decision
	}

	// 1. Filename/folder tokens (strong signals)
	webTokens := []string{"web-dl", "webrip", "webhd", "webdl", "nf", "amzn", "dsnp", "hmax", "hulu", "atvp", "disney", "appletv"}
	discTokens := []string{"bluray", "bdrip", "brrip", "remux", "uhd", "bd25", "bd50", "blu-ray", "bd-remux", "bd remux", "bdr"}

	// Check filename
	for _, token := range webTokens {
		if strings.Contains(fileName, token) {
			decision.Score += 3.0
			decision.Reasons = append(decision.Reasons, fmt.Sprintf("filename: contains '%s'", token))
		}
	}
	for _, token := range discTokens {
		if strings.Contains(fileName, token) {
			decision.Score -= 4.0 // Strong disc indicator
			decision.Reasons = append(decision.Reasons, fmt.Sprintf("filename: contains '%s'", token))
		}
	}

	// Check directory name (weaker signal)
	for _, token := range webTokens {
		if strings.Contains(dirName, token) {
			decision.Score += 1.0
			decision.Reasons = append(decision.Reasons, fmt.Sprintf("directory: contains '%s'", token))
		}
	}
	for _, token := range discTokens {
		if strings.Contains(dirName, token) {
			decision.Score -= 2.0
			decision.Reasons = append(decision.Reasons, fmt.Sprintf("directory: contains '%s'", token))
		}
	}

	// 2. Container & muxing info
	// File extension
	if ext == ".mp4" || ext == ".mov" || ext == ".webm" {
		decision.Score += 2.0
		decision.Reasons = append(decision.Reasons, fmt.Sprintf("extension: %s (web container)", ext))
	} else if ext == ".mkv" {
		decision.Score -= 1.0 // MKV leans disc-like
		decision.Reasons = append(decision.Reasons, "extension: .mkv (often disc remux)")
	}

	// Format name
	if formatName == "mov,mp4,m4a,3gp,3g2,mj2" || formatName == "mp4" || formatName == "mov" {
		decision.Score += 2.5
		decision.Reasons = append(decision.Reasons, fmt.Sprintf("format: %s (web container)", formatName))
	} else if strings.HasPrefix(formatName, "webm") && !strings.Contains(formatName, "matroska") {
		decision.Score += 2.5
		decision.Reasons = append(decision.Reasons, fmt.Sprintf("format: %s (web container)", formatName))
	} else if strings.Contains(formatName, "matroska") {
		decision.Score -= 1.5 // Matroska (MKV) leans disc-like
		decision.Reasons = append(decision.Reasons, "format: matroska (often disc remux)")
	}

	// Muxing app / writing library (strong signal)
	if format.Tags != nil {
		muxingApp := strings.ToLower(format.Tags["muxing_app"])
		writingLib := strings.ToLower(format.Tags["writing_library"])

		// Web-leaning muxers
		webMuxers := []string{"shaka-packager", "libwebm", "applehttp", "dash", "hls", "ffmpeg"}
		for _, muxer := range webMuxers {
			if strings.Contains(muxingApp, muxer) || strings.Contains(writingLib, muxer) {
				decision.Score += 3.0
				decision.Reasons = append(decision.Reasons, fmt.Sprintf("muxer: %s (web-leaning)", muxer))
			}
		}

		// Disc-leaning muxers
		discMuxers := []string{"mkvmerge", "libmatroska", "makemkv", "tsmuxer"}
		for _, muxer := range discMuxers {
			if strings.Contains(muxingApp, muxer) || strings.Contains(writingLib, muxer) {
				decision.Score -= 3.0
				decision.Reasons = append(decision.Reasons, fmt.Sprintf("muxer: %s (disc-leaning)", muxer))
			}
		}
	}

	// 3. Frame rate behavior (VFR is web-like)
	for _, stream := range streams {
		if stream.CodecType != "video" {
			continue
		}
		if stream.AvgFrameRate != "" && stream.RFrameRate != "" {
			if stream.AvgFrameRate != stream.RFrameRate {
				// Only count VFR if not Matroska (disc remuxes shouldn't have VFR)
				if !strings.Contains(formatName, "matroska") {
					decision.Score += 2.5
					decision.Reasons = append(decision.Reasons, fmt.Sprintf("video: VFR detected (avg=%s, r=%s)", stream.AvgFrameRate, stream.RFrameRate))
				}
				break
			}
		}
	}

	// 4. Dimensions & aspect ratio
	for _, stream := range streams {
		if stream.CodecType != "video" {
			continue
		}

		// Odd dimensions (web-like, but only if not Matroska)
		if !strings.Contains(formatName, "matroska") {
			if stream.Width > 0 && stream.Width%2 != 0 {
				decision.Score += 1.5
				decision.Reasons = append(decision.Reasons, fmt.Sprintf("video: odd width %d", stream.Width))
			}
			if stream.Height > 0 && stream.Height%2 != 0 {
				decision.Score += 1.5
				decision.Reasons = append(decision.Reasons, fmt.Sprintf("video: odd height %d", stream.Height))
			}
		}

		// Unusual aspect ratios (weak signal)
		if stream.Width > 0 && stream.Height > 0 {
			ar := float64(stream.Width) / float64(stream.Height)
			if ar < 1.3 || ar > 2.5 {
				decision.Score += 0.5
				decision.Reasons = append(decision.Reasons, fmt.Sprintf("video: unusual AR %.2f", ar))
			}
		}
	}

	// 5. Bitrate vs resolution (weak signal, only if we have both)
	if format.BitRate != "" {
		// Parse bitrate (in bps)
		bitrate, err := strconv.ParseFloat(format.BitRate, 64)
		if err == nil {
			// Find video stream for resolution
			for _, stream := range streams {
				if stream.CodecType == "video" && stream.Height > 0 {
					// Very low bitrate for resolution suggests web
					// Very high bitrate suggests disc
					bitsPerPixel := bitrate / float64(stream.Width*stream.Height)
					if bitsPerPixel < 0.1 && stream.Height >= 1080 {
						decision.Score += 1.0
						decision.Reasons = append(decision.Reasons, fmt.Sprintf("bitrate: low for resolution (%.2f bpp)", bitsPerPixel))
					} else if bitsPerPixel > 0.3 && stream.Height >= 1080 {
						decision.Score -= 1.0
						decision.Reasons = append(decision.Reasons, fmt.Sprintf("bitrate: high for resolution (%.2f bpp)", bitsPerPixel))
					}
					break
				}
			}
		}
	}

	// Determine final class based on score
	// Thresholds: >= +2.0 = WebLike, <= -2.0 = DiscLike, otherwise Unknown
	if decision.Score >= 2.0 {
		decision.Class = SourceWebLike
	} else if decision.Score <= -2.0 {
		decision.Class = SourceDiscLike
	} else {
		decision.Class = SourceUnknown
		decision.Reasons = append(decision.Reasons, "ambiguous: score near zero")
	}

	return decision
}

// WriteWhyFile writes a .av1qsvd-why.txt sidecar file explaining why a file was skipped or rejected.
// Uses new pattern to avoid conflicts with old .why.txt files.
func WriteWhyFile(filePath, reason string) error {
	basePath := strings.TrimSuffix(filePath, filepath.Ext(filePath))
	whyPath := basePath + ".av1qsvd-why.txt"
	return os.WriteFile(whyPath, []byte(reason), 0644)
}

// WriteClassificationInfo writes classification decision details to a sidecar file for debugging.
func WriteClassificationInfo(filePath string, decision *WebSourceDecision) error {
	if decision == nil {
		return nil
	}
	basePath := strings.TrimSuffix(filePath, filepath.Ext(filePath))
	infoPath := basePath + ".av1qsvd-classification.txt"
	
	lines := []string{
		fmt.Sprintf("Source Classification: %s", decision.Class.String()),
		fmt.Sprintf("Score: %.1f", decision.Score),
		"",
		"Reasons:",
	}
	for _, reason := range decision.Reasons {
		lines = append(lines, fmt.Sprintf("  - %s", reason))
	}
	
	return os.WriteFile(infoPath, []byte(strings.Join(lines, "\n")), 0644)
}
