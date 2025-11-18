package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// JobStatus represents the status of a transcoding job.
type JobStatus string

const (
	JobStatusPending JobStatus = "pending"
	JobStatusRunning JobStatus = "running"
	JobStatusSuccess JobStatus = "success"
	JobStatusFailed  JobStatus = "failed"
	JobStatusSkipped JobStatus = "skipped"
)

// Job represents a transcoding job.
type Job struct {
	ID           string     `json:"id"`
	SourcePath   string     `json:"source_path"`
	OutputPath   string     `json:"output_path,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	Status       JobStatus  `json:"status"`
	Reason       string     `json:"reason,omitempty"`
	OriginalSize int64      `json:"original_bytes,omitempty"`
	NewSize      int64      `json:"new_bytes,omitempty"`
	IsWebRipLike bool       `json:"is_webrip_like"`
}

// NewJob creates a new job with a generated ID and sets CreatedAt to now.
func NewJob(sourcePath string) *Job {
	return &Job{
		ID:           uuid.New().String(),
		SourcePath:   sourcePath,
		CreatedAt:    time.Now(),
		Status:       JobStatusPending,
		IsWebRipLike: false,
	}
}

// SaveJob saves a job to a JSON file in the jobs directory.
// The filename will be <job_id>.json
func SaveJob(job *Job, jobsDir string) error {
	// Ensure jobs directory exists
	if err := os.MkdirAll(jobsDir, 0755); err != nil {
		return fmt.Errorf("failed to create jobs directory: %w", err)
	}

	// Write job as JSON
	jobPath := filepath.Join(jobsDir, job.ID+".json")
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	if err := os.WriteFile(jobPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write job file: %w", err)
	}

	return nil
}

// LoadAllJobs loads all job JSON files from the jobs directory.
// Returns an empty slice if the directory doesn't exist or contains no jobs.
func LoadAllJobs(jobsDir string) ([]*Job, error) {
	// Check if directory exists
	if _, err := os.Stat(jobsDir); os.IsNotExist(err) {
		return []*Job{}, nil
	}

	// Read directory
	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read jobs directory: %w", err)
	}

	var jobs []*Job
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process .json files
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		jobPath := filepath.Join(jobsDir, entry.Name())
		data, err := os.ReadFile(jobPath)
		if err != nil {
			// Log but continue with other jobs
			continue
		}

		var job Job
		if err := json.Unmarshal(data, &job); err != nil {
			// Log but continue with other jobs
			continue
		}

		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// FindJobBySourcePath finds an existing job with the given source path.
func FindJobBySourcePath(jobs []*Job, sourcePath string) *Job {
	for _, job := range jobs {
		if job.SourcePath == sourcePath {
			return job
		}
	}
	return nil
}

