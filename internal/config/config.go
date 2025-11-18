package config

import (
	"os"
	"path/filepath"
)

// TranscodeConfig holds configuration for the AV1 transcoding daemon.
type TranscodeConfig struct {
	FFmpegURL        string   `json:"ffmpeg_url"`
	FFmpegInstallDir string   `json:"ffmpeg_install_dir"`
	LibraryRoots     []string `json:"library_roots"`
	MinBytes         int64    `json:"min_bytes"`          // e.g. 2 GiB
	MaxSizeRatio     float64  `json:"max_size_ratio"`     // e.g. 0.90
	JobStateDir      string   `json:"job_state_dir"`
	ScanIntervalSec  int      `json:"scan_interval_sec"`  // e.g. 60
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() TranscodeConfig {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home dir can't be determined
		homeDir = "."
	}

	dataDir := filepath.Join(homeDir, ".local", "share", "av1qsvd")
	ffmpegDir := filepath.Join(dataDir, "ffmpeg")
	jobsDir := filepath.Join(dataDir, "jobs")

	return TranscodeConfig{
		FFmpegURL:        "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n8.0-latest-linux64-gpl-8.0.tar.xz",
		FFmpegInstallDir: ffmpegDir,
		LibraryRoots:     []string{}, // Empty by default, to be configured
		MinBytes:         2 * 1024 * 1024 * 1024, // 2 GiB
		MaxSizeRatio:     0.90,
		JobStateDir:      jobsDir,
		ScanIntervalSec:  60,
	}
}

// LoadConfig loads configuration from a file path.
// For v1, this is a stub that returns DefaultConfig() and ignores the path.
// Future versions can read from JSON or TOML.
func LoadConfig(path string) (TranscodeConfig, error) {
	// TODO: Implement actual config file loading
	return DefaultConfig(), nil
}

