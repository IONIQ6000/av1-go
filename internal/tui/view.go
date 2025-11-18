package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/yourname/av1qsvd/internal/jobs"
)

var (
	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("220"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	failedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	skippedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226"))

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	pendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// View renders the TUI.
func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	var sections []string

	// Title
	sections = append(sections, titleStyle.Render("AV1 Transcoding Daemon"))

	// System metrics
	sections = append(sections, renderMetrics(m.cpuPercent, m.memPercent, m.gpuPercent))

	// Job table
	sections = append(sections, renderJobTable(m.jobs, m.width))

	// Status bar
	sections = append(sections, renderStatusBar(m.jobs, m.jobsDir, m.lastRefresh, m.width-2))

	return strings.Join(sections, "\n\n")
}

// renderMetrics renders CPU, memory, and GPU usage bars.
func renderMetrics(cpuPercent, memPercent, gpuPercent float64) string {
	cpuBar := renderBar("CPU", cpuPercent, 100)
	memBar := renderBar("MEM", memPercent, 100)
	gpuBar := renderBar("GPU", gpuPercent, 100)
	return lipgloss.JoinHorizontal(lipgloss.Left, cpuBar, "  ", memBar, "  ", gpuBar)
}

// renderBar renders a progress bar.
func renderBar(label string, value, max float64) string {
	width := 20
	filled := int((value / max) * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("%s: %s %.1f%%", label, bar, value)
}

// renderJobTable renders the job table.
func renderJobTable(jobs []*jobs.Job, width int) string {
	if len(jobs) == 0 {
		return "No jobs found"
	}

	// Calculate column widths
	colWidths := calculateColumnWidths(width)

	// Header
	header := renderRow(
		[]string{"STATUS", "FILE", "ORIG_SIZE", "NEW_SIZE", "SAVINGS", "DURATION", "REASON"},
		colWidths,
		true,
	)

	// Rows
	var rows []string
	rows = append(rows, headerStyle.Render(header))

	for _, job := range jobs {
		row := renderJobRow(job, colWidths)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

// formatDuration formats job duration.
func formatDuration(job *jobs.Job) string {
	if job.StartedAt == nil {
		return "-"
	}
	var endTime time.Time
	if job.FinishedAt != nil {
		endTime = *job.FinishedAt
	} else {
		endTime = time.Now()
	}
	duration := endTime.Sub(*job.StartedAt)
	if duration < time.Second {
		return "<1s"
	}
	if duration < time.Minute {
		return fmt.Sprintf("%.0fs", duration.Seconds())
	}
	return fmt.Sprintf("%.1fm", duration.Minutes())
}

// formatSize formats file size.
func formatSize(bytes int64) string {
	if bytes == 0 {
		return "-"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// calculateSavings calculates savings percentage.
func calculateSavings(origSize, newSize int64) string {
	if origSize == 0 || newSize == 0 {
		return "-"
	}
	savings := float64(origSize-newSize) / float64(origSize) * 100
	if savings < 0 {
		return fmt.Sprintf("+%.1f%%", -savings)
	}
	return fmt.Sprintf("%.1f%%", savings)
}

// calculateColumnWidths calculates column widths based on available width.
func calculateColumnWidths(totalWidth int) map[string]int {
	// Fixed widths for some columns
	widths := map[string]int{
		"STATUS":   8,
		"ORIG_SIZE": 10,
		"NEW_SIZE":  10,
		"SAVINGS":  8,
		"DURATION": 8,
		"REASON":   20,
	}

	// Calculate FILE column width (remaining space)
	usedWidth := widths["STATUS"] + widths["ORIG_SIZE"] + widths["NEW_SIZE"] +
		widths["SAVINGS"] + widths["DURATION"] + widths["REASON"] + 6 // separators
	fileWidth := totalWidth - usedWidth - 2 // padding
	if fileWidth < 10 {
		fileWidth = 10
	}
	widths["FILE"] = fileWidth

	return widths
}

// renderRow renders a table row.
func renderRow(columns []string, widths map[string]int, isHeader bool) string {
	colNames := []string{"STATUS", "FILE", "ORIG_SIZE", "NEW_SIZE", "SAVINGS", "DURATION", "REASON"}
	var parts []string
	for i, colName := range colNames {
		width := widths[colName]
		text := ""
		if i < len(columns) {
			text = columns[i]
		}
		// Truncate or pad
		if len(text) > width {
			text = text[:width-3] + "..."
		} else {
			text = text + strings.Repeat(" ", width-len(text))
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, " ")
}

// renderJobRow renders a job row.
func renderJobRow(job *jobs.Job, widths map[string]int) string {
	status := formatStatus(job.Status)
	fileName := filepath.Base(job.SourcePath)
	origSize := formatSize(job.OriginalSize)
	newSize := formatSize(job.NewSize)
	savings := calculateSavings(job.OriginalSize, job.NewSize)
	duration := formatDuration(job)
	reason := job.Reason
	if reason == "" {
		reason = "-"
	}

	row := renderRow(
		[]string{status, fileName, origSize, newSize, savings, duration, reason},
		widths,
		false,
	)

	// Apply color based on status
	switch job.Status {
	case jobs.JobStatusSuccess:
		return successStyle.Render(row)
	case jobs.JobStatusFailed:
		return failedStyle.Render(row)
	case jobs.JobStatusSkipped:
		return skippedStyle.Render(row)
	case jobs.JobStatusRunning:
		return runningStyle.Render(row)
	case jobs.JobStatusPending:
		return pendingStyle.Render(row)
	default:
		return row
	}
}

// formatStatus formats job status.
func formatStatus(status jobs.JobStatus) string {
	switch status {
	case jobs.JobStatusPending:
		return "PENDING"
	case jobs.JobStatusRunning:
		return "RUNNING"
	case jobs.JobStatusSuccess:
		return "SUCCESS"
	case jobs.JobStatusFailed:
		return "FAILED"
	case jobs.JobStatusSkipped:
		return "SKIPPED"
	default:
		return string(status)
	}
}

// renderStatusBar renders the status bar at the bottom.
func renderStatusBar(jobList []*jobs.Job, jobsDir string, lastRefresh time.Time, width int) string {
	var stats struct {
		total   int
		running int
		failed  int
		skipped int
	}

	for _, job := range jobList {
		stats.total++
		switch job.Status {
		case jobs.JobStatusRunning:
			stats.running++
		case jobs.JobStatusFailed:
			stats.failed++
		case jobs.JobStatusSkipped:
			stats.skipped++
		}
	}

	statusText := fmt.Sprintf("Jobs: %d total | %d running | %d failed | %d skipped | Dir: %s | Last refresh: %s | Press 'q' to quit, 'r' to refresh",
		stats.total,
		stats.running,
		stats.failed,
		stats.skipped,
		jobsDir,
		lastRefresh.Format("15:04:05"),
	)

	// Truncate if too long
	if len(statusText) > width {
		statusText = statusText[:width-3] + "..."
	}

	return statusBarStyle.Render(statusText)
}
