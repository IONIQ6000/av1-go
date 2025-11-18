package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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
		log.Fatalf("Failed to ensure ffmpeg: %v", err)
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

			// Check for .av1skip marker
			skipMarker := strings.TrimSuffix(path, ext) + ".av1skip"
			if _, err := os.Stat(skipMarker); err == nil {
				reason := "marked with .av1skip"
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
				// Skip files that already have jobs (unless they're pending and need re-evaluation)
				if existingJob.Status != jobs.JobStatusPending {
					return nil
				}
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
			log.Printf("  → Running ffprobe...")
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
			} else {
				job = jobs.NewJob(path)
			}

			job.OriginalSize = info.Size()
			job.IsWebRipLike = probeResult.IsWebRipLike
			
			// Populate metadata from probe result
			if probeResult.VideoStream != nil {
				job.SourceCodec = probeResult.VideoStream.CodecName
				job.Resolution = fmt.Sprintf("%dx%d", probeResult.VideoStream.Width, probeResult.VideoStream.Height)
				job.BitDepth = probeResult.VideoStream.BitDepth
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
			
			// Estimate output size (rough estimate: AV1 is typically 50% of original for similar quality)
			job.EstimatedSize = int64(float64(info.Size()) * 0.5)

			// Save job
			if err := jobs.SaveJob(job, cfg.JobStateDir); err != nil {
				log.Printf("Failed to save job for %s: %v", path, err)
				return nil
			}

			candidates = append(candidates, path)
			newJobs = append(newJobs, job)
			log.Printf("  → ✓ ACCEPTED: %s (WebRip-like: %v, codec: %s, resolution: %s)", 
				path, probeResult.IsWebRipLike, job.SourceCodec, job.Resolution)

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

type skippedFile struct {
	path   string
	reason string
}

