package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yourname/av1qsvd/internal/config"
	"github.com/yourname/av1qsvd/internal/daemon"
	"github.com/yourname/av1qsvd/internal/ffmpeg"
	"github.com/yourname/av1qsvd/internal/jobs"
	"github.com/yourname/av1qsvd/internal/metadata"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Load configuration
	// Try to load from /etc/av1qsvd/config.json, fallback to default
	configPath := "/etc/av1qsvd/config.json"
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Printf("Failed to load config from %s, using defaults: %v", configPath, err)
		cfg = config.DefaultConfig()
	}
	log.Printf("Using config: FFmpeg install dir: %s, Job state dir: %s", cfg.FFmpegInstallDir, cfg.JobStateDir)
	log.Printf("Library roots configured: %d", len(cfg.LibraryRoots))
	for i, root := range cfg.LibraryRoots {
		log.Printf("  [%d] %s", i+1, root)
	}
	log.Printf("Min file size: %d bytes (%.2f GB)", cfg.MinBytes, float64(cfg.MinBytes)/(1024*1024*1024))

	// Ensure ffmpeg is installed and verified
	ffmpegPath, err := ffmpeg.EnsureFFmpeg(cfg.FFmpegInstallDir, cfg.FFmpegURL)
	if err != nil {
		// Check if it's a QSV test failure - allow daemon to start anyway
		// QSV will be tested again during actual transcoding
		if strings.Contains(err.Error(), "QSV test failed") || strings.Contains(err.Error(), "GPU device not accessible") {
			log.Printf("Warning: QSV test failed during startup: %v", err)
			log.Printf("Daemon will start anyway - QSV will be tested during transcoding")
			log.Printf("If transcoding fails, check GPU permissions and drivers")
			// Ensure ffmpegPath is set even if QSV test failed
			if ffmpegPath == "" {
				ffmpegPath = filepath.Join(cfg.FFmpegInstallDir, "ffmpeg")
			}
			if _, err := os.Stat(ffmpegPath); err != nil {
				log.Fatalf("ffmpeg binary not found at %s: %v", ffmpegPath, err)
			}
			log.Printf("Using ffmpeg at %s (QSV test failed but binary exists)", ffmpegPath)
		} else {
			log.Fatalf("Failed to ensure ffmpeg: %v", err)
		}
	}

	// Validate ffmpegPath is set and executable
	if ffmpegPath == "" {
		log.Fatalf("ffmpeg path is empty!")
	}
	if _, err := os.Stat(ffmpegPath); err != nil {
		log.Fatalf("ffmpeg binary not found at %s: %v", ffmpegPath, err)
	}
	log.Printf("ffmpeg ready at: %s", ffmpegPath)

	// Load existing jobs
	existingJobs, err := jobs.LoadAllJobs(cfg.JobStateDir)
	if err != nil {
		log.Printf("Warning: failed to load existing jobs: %v", err)
		existingJobs = []*jobs.Job{}
	}
	log.Printf("Loaded %d existing jobs", len(existingJobs))

	// Perform a single scan pass
	if len(cfg.LibraryRoots) == 0 {
		log.Printf("No library roots configured. Use DefaultConfig() and set LibraryRoots to scan directories.")
		return
	}

	var candidates []string
	var skipped []skippedFile
	var newJobs []*jobs.Job

	for _, root := range cfg.LibraryRoots {
		log.Printf("Scanning library root: %s", root)
		if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("Error accessing %s: %v", path, err)
				return nil // Continue walking
			}

			if info.IsDir() {
				return nil
			}

			// Check if file has a media extension
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".mkv" && ext != ".mp4" && ext != ".m4v" {
				return nil
			}
			log.Printf("Found media file: %s (ext: %s, size: %.2f GB)", path, ext, float64(info.Size())/(1024*1024*1024))

			// Check for .av1qsvd-skip marker (new pattern to avoid old .av1skip conflicts)
			skipMarker := strings.TrimSuffix(path, ext) + ".av1qsvd-skip"
			if _, err := os.Stat(skipMarker); err == nil {
				reason := "marked with .av1qsvd-skip"
				skipped = append(skipped, skippedFile{
					path:   path,
					reason: reason,
				})
				metadata.WriteWhyFile(path, reason)
				return nil
			}

			// Check if job already exists for this file
			existingJob := jobs.FindJobBySourcePath(existingJobs, path)
			if existingJob != nil {
				// Only skip if job succeeded (already transcoded)
				// Ignore old skipped/failed jobs - re-evaluate them
				if existingJob.Status == jobs.JobStatusSuccess {
					log.Printf("  → Skipped: already successfully transcoded (job %s)", existingJob.ID)
					return nil
				}
				// For pending/running/skipped/failed jobs, continue to re-evaluate
				// This allows files to be re-scanned if they were previously skipped/failed
			}

			// Check file size
			if info.Size() <= cfg.MinBytes {
				reason := fmt.Sprintf("file < 2GB (size: %d bytes, %.2f GB)", info.Size(), float64(info.Size())/(1024*1024*1024))
				log.Printf("  → Skipped: %s", reason)
				skipped = append(skipped, skippedFile{
					path:   path,
					reason: reason,
				})
				metadata.WriteWhyFile(path, reason)
				return nil
			}
			log.Printf("  → File size OK: %.2f GB", float64(info.Size())/(1024*1024*1024))

			// Run ffprobe to get metadata
			log.Printf("  → Running ffprobe... (ffmpegPath: %q)", ffmpegPath)
			probeResult, err := metadata.ProbeFile(ffmpegPath, path)
			if err != nil {
				reason := fmt.Sprintf("ffprobe failed: %v", err)
				log.Printf("  → Skipped: %s", reason)
				skipped = append(skipped, skippedFile{
					path:   path,
					reason: reason,
				})
				metadata.WriteWhyFile(path, reason)
				return nil
			}

			// Check if it's a video
			if !probeResult.HasVideo {
				reason := "not a video"
				log.Printf("  → Skipped: %s", reason)
				skipped = append(skipped, skippedFile{
					path:   path,
					reason: reason,
				})
				metadata.WriteWhyFile(path, reason)
				return nil
			}
			log.Printf("  → Video detected: codec=%s, resolution=%dx%d",
				probeResult.VideoStream.CodecName,
				probeResult.VideoStream.Width,
				probeResult.VideoStream.Height)

			// Check if already AV1
			if probeResult.HasAV1 {
				reason := "already av1"
				log.Printf("  → Skipped: %s", reason)
				skipped = append(skipped, skippedFile{
					path:   path,
					reason: reason,
				})
				metadata.WriteWhyFile(path, reason)
				return nil
			}

			// File passed all checks - create or update job
			var job *jobs.Job
			if existingJob != nil {
				job = existingJob
				// Reset status to pending if it was previously skipped/failed
				// This allows re-processing of files that were previously rejected
				if job.Status == jobs.JobStatusSkipped || job.Status == jobs.JobStatusFailed {
					log.Printf("  → Resetting old %s job to pending for re-evaluation", job.Status)
					job.Status = jobs.JobStatusPending
					job.Reason = "" // Clear old reason
					job.StartedAt = nil
					job.FinishedAt = nil
				}
			} else {
				job = jobs.NewJob(path)
			}

			job.OriginalSize = info.Size()
			job.IsWebRipLike = probeResult.IsWebRipLike

			// Populate metadata from probe result
			if probeResult.VideoStream != nil {
				job.SourceCodec = probeResult.VideoStream.CodecName
				job.Resolution = fmt.Sprintf("%dx%d", probeResult.VideoStream.Width, probeResult.VideoStream.Height)
				job.BitDepth = int(probeResult.VideoStream.BitDepth)
				job.FrameRate = probeResult.VideoStream.AvgFrameRate
				if job.FrameRate == "" {
					job.FrameRate = probeResult.VideoStream.RFrameRate
				}
			}

			// Count streams
			audioCount := 0
			subCount := 0
			for _, stream := range probeResult.Streams {
				switch stream.CodecType {
				case "audio":
					audioCount++
				case "subtitle":
					subCount++
				}
			}
			job.AudioStreams = audioCount
			job.SubStreams = subCount

			// Container from format
			job.Container = probeResult.Format.FormatName

			// Calculate estimated output size based on bitrate analysis
			quality := 24 // default
			if probeResult.VideoStream != nil {
				quality = ffmpeg.DetermineQuality(probeResult.VideoStream.Height)
			}
			job.EstimatedSize = estimateOutputSize(info.Size(), probeResult, quality)

			// Save job
			if err := jobs.SaveJob(job, cfg.JobStateDir); err != nil {
				log.Printf("Failed to save job for %s: %v", path, err)
				return nil
			}

			candidates = append(candidates, path)
			newJobs = append(newJobs, job)
			// Log classification decision with details
			if probeResult.SourceDecision != nil {
				log.Printf("  → ✓ ACCEPTED: %s (source: %s, score: %.1f, codec: %s, resolution: %s)",
					path, probeResult.SourceDecision.Class.String(), probeResult.SourceDecision.Score, job.SourceCodec, job.Resolution)
				if len(probeResult.SourceDecision.Reasons) > 0 {
					log.Printf("    Classification reasons: %s", strings.Join(probeResult.SourceDecision.Reasons, "; "))
				}
				// Write classification info to sidecar file for debugging
				if err := metadata.WriteClassificationInfo(path, probeResult.SourceDecision); err != nil {
					log.Printf("  Warning: failed to write classification info: %v", err)
				}
			} else {
				log.Printf("  → ✓ ACCEPTED: %s (WebRip-like: %v, codec: %s, resolution: %s)",
					path, probeResult.IsWebRipLike, job.SourceCodec, job.Resolution)
			}

			return nil
		}); err != nil {
			log.Printf("Error walking directory %s: %v", root, err)
		}
	}

	// Print summary
	fmt.Println("\n=== Scan Summary ===")
	fmt.Printf("Candidates (queued as jobs): %d\n", len(candidates))
	for _, path := range candidates {
		fmt.Printf("  [CANDIDATE] %s\n", path)
	}

	fmt.Printf("\nSkipped files: %d\n", len(skipped))
	for _, sf := range skipped {
		fmt.Printf("  [SKIPPED] %s - reason: %s\n", sf.path, sf.reason)
	}

	fmt.Println("\n=== Scan Complete ===")

	fmt.Printf("\nCreated/updated %d jobs\n", len(newJobs))

	// Process pending jobs (one at a time for v1)
	pendingJobs := []*jobs.Job{}
	for _, job := range existingJobs {
		if job.Status == jobs.JobStatusPending {
			pendingJobs = append(pendingJobs, job)
		}
	}
	for _, job := range newJobs {
		if job.Status == jobs.JobStatusPending {
			pendingJobs = append(pendingJobs, job)
		}
	}

	if len(pendingJobs) == 0 {
		log.Printf("No pending jobs to process")
		return
	}

	log.Printf("Processing %d pending jobs...", len(pendingJobs))

	// Process jobs one at a time
	for _, job := range pendingJobs {
		log.Printf("Processing job %s: %s", job.ID, job.SourcePath)

		// Re-probe file to get fresh metadata
		probeResult, err := metadata.ProbeFile(ffmpegPath, job.SourcePath)
		if err != nil {
			log.Printf("Failed to probe file %s: %v", job.SourcePath, err)
			job.Status = jobs.JobStatusFailed
			job.Reason = fmt.Sprintf("ffprobe failed: %v", err)
			jobs.SaveJob(job, cfg.JobStateDir)
			continue
		}

		// Update job with fresh metadata
		job.IsWebRipLike = probeResult.IsWebRipLike

		// Process the job
		daemonCfg := daemon.TranscodeConfig{
			JobStateDir:  cfg.JobStateDir,
			MaxSizeRatio: cfg.MaxSizeRatio,
		}

		if err := daemon.ProcessJob(job, ffmpegPath, probeResult, daemonCfg); err != nil {
			log.Printf("Job %s failed: %v", job.ID, err)
			continue
		}

		// Log result
		switch job.Status {
		case jobs.JobStatusSuccess:
			savings := float64(job.OriginalSize-job.NewSize) / float64(job.OriginalSize) * 100
			log.Printf("Job succeeded: %s - savings: %.1f%%", job.SourcePath, savings)
		case jobs.JobStatusSkipped:
			log.Printf("Job skipped: %s - reason: %s", job.SourcePath, job.Reason)
		case jobs.JobStatusFailed:
			log.Printf("Job failed: %s - reason: %s", job.SourcePath, job.Reason)
		}
	}

	log.Printf("Finished processing jobs")
}

// estimateOutputSize calculates estimated output size based on actual bitrate analysis
func estimateOutputSize(originalSize int64, probeResult *metadata.ProbeResult, quality int) int64 {
	if probeResult.VideoStream == nil {
		return 0
	}

	// Parse duration
	duration, err := strconv.ParseFloat(probeResult.Format.Duration, 64)
	if err != nil || duration <= 0 {
		return 0
	}

	// Parse total bitrate (bits per second)
	totalBitrate, err := strconv.ParseFloat(probeResult.Format.BitRate, 64)
	if err != nil || totalBitrate <= 0 {
		return 0
	}

	// Calculate video bitrate (subtract audio/subtitle bitrates)
	videoBitrate := totalBitrate
	for _, stream := range probeResult.Streams {
		if stream.CodecType == "audio" || stream.CodecType == "subtitle" {
			if stream.BitRate != "" {
				if streamBR, err := strconv.ParseFloat(stream.BitRate, 64); err == nil {
					videoBitrate -= streamBR
				}
			}
		}
	}

	// If we couldn't parse stream bitrates, estimate audio overhead
	// Typical: 1-2 audio streams at 192-384 kbps each, subtitles negligible
	if videoBitrate >= totalBitrate*0.95 {
		// Assume ~5% overhead for audio if we couldn't parse it
		videoBitrate = totalBitrate * 0.95
	}

	// Estimate AV1 video bitrate based on quality, resolution, and frame rate
	videoStream := probeResult.VideoStream
	pixels := float64(videoStream.Width * videoStream.Height)

	// Parse frame rate
	fps := 24.0 // default
	if videoStream.AvgFrameRate != "" {
		// Parse "24000/1001" format
		parts := strings.Split(videoStream.AvgFrameRate, "/")
		if len(parts) == 2 {
			num, err1 := strconv.ParseFloat(parts[0], 64)
			den, err2 := strconv.ParseFloat(parts[1], 64)
			if err1 == nil && err2 == nil && den > 0 {
				fps = num / den
			}
		} else {
			if f, err := strconv.ParseFloat(videoStream.AvgFrameRate, 64); err == nil {
				fps = f
			}
		}
	}

	// Estimate AV1 bitrate based on quality setting
	// Quality 23 (4K): ~0.15 bits per pixel per frame
	// Quality 24 (1080p): ~0.12 bits per pixel per frame
	// Quality 25 (<1080p): ~0.10 bits per pixel per frame
	var bitsPerPixelPerFrame float64
	switch quality {
	case 23:
		bitsPerPixelPerFrame = 0.15
	case 24:
		bitsPerPixelPerFrame = 0.12
	case 25:
		bitsPerPixelPerFrame = 0.10
	default:
		bitsPerPixelPerFrame = 0.12
	}

	// Calculate estimated AV1 video bitrate
	estimatedAV1VideoBitrate := pixels * bitsPerPixelPerFrame * fps

	// Calculate compression ratio
	compressionRatio := estimatedAV1VideoBitrate / videoBitrate

	// Estimate video size reduction
	// Video portion of original file
	originalVideoSize := int64(float64(originalSize) * (videoBitrate / totalBitrate))

	// Estimated AV1 video size
	estimatedAV1VideoSize := int64(float64(originalVideoSize) * compressionRatio)

	// Audio/subtitle sizes stay the same (they're copied)
	audioSubtitleSize := originalSize - originalVideoSize

	// Estimated total size
	estimatedTotalSize := estimatedAV1VideoSize + audioSubtitleSize

	// Add container overhead (~1-2% for Matroska)
	estimatedTotalSize = int64(float64(estimatedTotalSize) * 1.02)

	// Ensure estimate is reasonable (not negative, not larger than original)
	if estimatedTotalSize <= 0 {
		return 0
	}
	if estimatedTotalSize > originalSize {
		// If estimate is larger, cap at 95% of original (conservative)
		estimatedTotalSize = int64(float64(originalSize) * 0.95)
	}

	return estimatedTotalSize
}

type skippedFile struct {
	path   string
	reason string
}
