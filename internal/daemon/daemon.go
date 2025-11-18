package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourname/av1qsvd/internal/ffmpeg"
	"github.com/yourname/av1qsvd/internal/jobs"
	"github.com/yourname/av1qsvd/internal/metadata"
	"github.com/yourname/av1qsvd/internal/scan"
)

// CheckSizeGate checks if the new file passes the size gate.
// Returns true if newBytes <= origBytes * maxRatio, false otherwise.
func CheckSizeGate(origBytes, newBytes int64, maxRatio float64) bool {
	maxAllowed := float64(origBytes) * maxRatio
	return float64(newBytes) <= maxAllowed
}

// AtomicReplaceFile atomically replaces the original file with the new file.
// Writes to a temporary file first, then renames it to the original.
func AtomicReplaceFile(originalPath, newPath string) error {
	// Get directory and base name
	dir := filepath.Dir(originalPath)
	baseName := filepath.Base(originalPath)
	ext := filepath.Ext(baseName)
	baseWithoutExt := strings.TrimSuffix(baseName, ext)

	// Create temporary output path
	tmpPath := filepath.Join(dir, baseWithoutExt+".av1-tmp.mkv")

	// Move new file to temp location (if not already there)
	if newPath != tmpPath {
		if err := os.Rename(newPath, tmpPath); err != nil {
			return fmt.Errorf("failed to move new file to temp: %w", err)
		}
	}

	// Verify temp file exists and is valid
	if _, err := os.Stat(tmpPath); err != nil {
		return fmt.Errorf("temp file does not exist: %w", err)
	}

	// Atomically replace original with temp file
	if err := os.Rename(tmpPath, originalPath); err != nil {
		return fmt.Errorf("failed to replace original file: %w", err)
	}

	return nil
}

// ProcessJob processes a single transcoding job.
// This function handles the full lifecycle: stability check, transcoding, size gate, and file replacement.
func ProcessJob(job *jobs.Job, ffmpegPath string, probeResult *metadata.ProbeResult, cfg TranscodeConfig) error {
	// Check file stability before starting
	stable, err := scan.CheckFileStable(job.SourcePath, 10)
	if err != nil {
		return fmt.Errorf("failed to check file stability: %w", err)
	}
	if !stable {
		reason := "file still copying"
		job.Status = jobs.JobStatusSkipped
		job.Reason = reason
		now := time.Now()
		job.FinishedAt = &now
		metadata.WriteWhyFile(job.SourcePath, reason)
		return nil // Not an error, just skip for now
	}

	// Mark job as running
	now := time.Now()
	job.Status = jobs.JobStatusRunning
	job.StartedAt = &now
	if err := jobs.SaveJob(job, cfg.JobStateDir); err != nil {
		return fmt.Errorf("failed to save job status: %w", err)
	}

	// Build output path
	dir := filepath.Dir(job.SourcePath)
	baseName := filepath.Base(job.SourcePath)
	ext := filepath.Ext(baseName)
	baseWithoutExt := strings.TrimSuffix(baseName, ext)
	outputPath := filepath.Join(dir, baseWithoutExt+".av1-tmp.mkv")
	job.OutputPath = outputPath

	// Build ffmpeg command
	args, err := ffmpeg.TranscodeArgs(ffmpegPath, job.SourcePath, outputPath, probeResult, job.IsWebRipLike)
	if err != nil {
		job.Status = jobs.JobStatusFailed
		job.Reason = fmt.Sprintf("failed to build ffmpeg args: %v", err)
		now := time.Now()
		job.FinishedAt = &now
		jobs.SaveJob(job, cfg.JobStateDir)
		return fmt.Errorf("failed to build transcode args: %w", err)
	}

	// Run transcode
	exitCode, err := ffmpeg.RunTranscode(ffmpegPath, args)
	if err != nil {
		job.Status = jobs.JobStatusFailed
		job.Reason = fmt.Sprintf("ffmpeg exit code %d: %v", exitCode, err)
		now := time.Now()
		job.FinishedAt = &now
		jobs.SaveJob(job, cfg.JobStateDir)
		metadata.WriteWhyFile(job.SourcePath, job.Reason)
		// Clean up output file if it exists
		os.Remove(outputPath)
		return fmt.Errorf("transcode failed: %w", err)
	}

	// Check output file size
	outputInfo, err := os.Stat(outputPath)
	if err != nil {
		job.Status = jobs.JobStatusFailed
		job.Reason = fmt.Sprintf("failed to stat output file: %v", err)
		now := time.Now()
		job.FinishedAt = &now
		jobs.SaveJob(job, cfg.JobStateDir)
		os.Remove(outputPath)
		return fmt.Errorf("output file not found: %w", err)
	}

	job.NewSize = outputInfo.Size()

	// Check size gate
	if !CheckSizeGate(job.OriginalSize, job.NewSize, cfg.MaxSizeRatio) {
		// Size gate failed - reject
		reason := fmt.Sprintf("size gate: new %.1f MB vs orig %.1f MB (>%.0f%%)",
			float64(job.NewSize)/(1024*1024),
			float64(job.OriginalSize)/(1024*1024),
			cfg.MaxSizeRatio*100)
		job.Status = jobs.JobStatusSkipped
		job.Reason = reason
		now := time.Now()
		job.FinishedAt = &now

		// Write .av1qsvd-why.txt and .av1qsvd-skip
		metadata.WriteWhyFile(job.SourcePath, reason)
		skipMarker := strings.TrimSuffix(job.SourcePath, ext) + ".av1qsvd-skip"
		os.WriteFile(skipMarker, []byte("skip"), 0644)

		// Delete output file
		os.Remove(outputPath)
		jobs.SaveJob(job, cfg.JobStateDir)
		return nil // Not an error, just rejected
	}

	// Size gate passed - atomically replace original
	// AtomicReplaceFile will replace the original with the new file
	// The original file is effectively deleted/replaced in this operation
	if err := AtomicReplaceFile(job.SourcePath, outputPath); err != nil {
		job.Status = jobs.JobStatusFailed
		job.Reason = fmt.Sprintf("failed to replace file: %v", err)
		now := time.Now()
		job.FinishedAt = &now
		jobs.SaveJob(job, cfg.JobStateDir)
		os.Remove(outputPath)
		return fmt.Errorf("failed to replace file: %w", err)
	}

	// Verify the replacement succeeded by checking the file exists
	if _, err := os.Stat(job.SourcePath); err != nil {
		job.Status = jobs.JobStatusFailed
		job.Reason = fmt.Sprintf("replaced file verification failed: %v", err)
		now := time.Now()
		job.FinishedAt = &now
		jobs.SaveJob(job, cfg.JobStateDir)
		return fmt.Errorf("replaced file verification failed: %w", err)
	}

	// All verification checks passed - original file has been replaced
	// Success!
	job.Status = jobs.JobStatusSuccess
	now = time.Now()
	job.FinishedAt = &now
	jobs.SaveJob(job, cfg.JobStateDir)

	return nil
}

// TranscodeConfig is a subset of config needed for job processing.
type TranscodeConfig struct {
	JobStateDir  string
	MaxSizeRatio float64
}
