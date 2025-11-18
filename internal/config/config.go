package config

import (
	"encoding/json"
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

// LoadConfig loads configuration from a JSON file path.
// If the file doesn't exist or can't be read, returns an error.
// Callers should fall back to DefaultConfig() if needed.
func LoadConfig(path string) (TranscodeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TranscodeConfig{}, err
	}

	var cfg TranscodeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return TranscodeConfig{}, err
	}

	return cfg, nil
}

