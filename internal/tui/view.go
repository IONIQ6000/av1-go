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

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2).
			Margin(1, 1, 0, 0)

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("213"))

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	summaryLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))

	summaryValueStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("79"))

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

	title := titleStyle.Render("AV1 Transcoding Daemon - Detailed View")

	metricsWidth := maxInt(32, m.width/2-3)
	if metricsWidth > 48 {
		metricsWidth = 48
	}
	summaryWidth := maxInt(32, m.width-metricsWidth-6)
	
	metricsBody := renderMetrics(m.cpuPercent, m.memPercent, m.gpuPercent)
	summaryBody := renderQueueSummary(m.jobs)
	
	// Ensure both panels have the same height by padding the shorter one
	metricsLines := strings.Count(metricsBody, "\n") + 1
	summaryLines := strings.Count(summaryBody, "\n") + 1
	maxLines := maxInt(metricsLines, summaryLines)
	
	// Pad metrics if needed
	if metricsLines < maxLines {
		metricsBody += strings.Repeat("\n", maxLines-metricsLines)
	}
	// Pad summary if needed
	if summaryLines < maxLines {
		summaryBody += strings.Repeat("\n", maxLines-summaryLines)
	}
	
	metricsPanel := renderPanel("SYSTEM METRICS", metricsBody, metricsWidth)
	summaryPanel := renderPanel("QUEUE SUMMARY", summaryBody, summaryWidth)
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, metricsPanel, summaryPanel)

	activeBody, hasActive := renderActiveJob(m.jobs)
	if !hasActive {
		activeBody = mutedStyle.Render("No jobs are currently running.")
	}
	activePanel := renderPanel("ACTIVE TRANSCODE", activeBody, m.width-4)

	statusBar := renderStatusBar(m.jobs, m.jobsDir, m.lastRefresh, m.width-2)

	tableWidth := maxInt(80, m.width-8)

	// Calculate how many lines we can devote to the job table body
	titleHeight := lipgloss.Height(title)
	topRowHeight := lipgloss.Height(topRow)
	activeHeight := lipgloss.Height(activePanel)
	statusHeight := lipgloss.Height(statusBar)
	availableBody := m.height - (titleHeight + topRowHeight + activeHeight + statusHeight) - 4
	if availableBody < 8 {
		availableBody = 8
	}

	// Account for panel border/padding overhead (roughly 6 lines in total)
	panelOverhead := 6
	maxJobLines := availableBody - panelOverhead
	if maxJobLines < 5 {
		maxJobLines = 5
	}

	jobsPanel := renderPanel("JOB QUEUE", renderJobTable(m.jobs, tableWidth, maxJobLines), m.width-2)

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		topRow,
		activePanel,
		jobsPanel,
		statusBar,
	)
}

// renderPanel wraps content inside a floating panel with a title.
func renderPanel(title, body string, width int) string {
	content := panelTitleStyle.Render(title)
	if body != "" {
		content += "\n" + body
	} else {
		content += "\n" + mutedStyle.Render("—")
	}

	if width > 0 {
		return panelStyle.Width(width).Render(content)
	}
	return panelStyle.Render(content)
}

// renderMetrics renders CPU, memory, and GPU usage bars.
func renderMetrics(cpuPercent, memPercent, gpuPercent float64) string {
	lines := []string{
		renderBar("CPU", cpuPercent, 24),
		renderBar("MEM", memPercent, 24),
		renderBar("GPU", gpuPercent, 24),
	}
	return strings.Join(lines, "\n")
}

// renderBar renders a progress bar (all bars now use same format).
func renderBar(label string, value float64, width int) string {
	filled := int((value / 100.0) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("%s: %s %5.1f%%", label, bar, value)
}

// renderActiveJob renders detailed information about the currently active job.
func renderActiveJob(jobList []*jobs.Job) (string, bool) {
	var runningJob *jobs.Job
	for _, job := range jobList {
		if job.Status == jobs.JobStatusRunning {
			runningJob = job
			break
		}
	}

	if runningJob == nil {
		return "", false
	}

	var details []string

	// Header
	details = append(details, headerStyle.Render("⚡ ACTIVE TRANSCODE"))

	// File information
	fileName := filepath.Base(runningJob.SourcePath)
	details = append(details, fmt.Sprintf("File: %s", fileName))

	// Source details
	if runningJob.Resolution != "" {
		details = append(details, fmt.Sprintf("Resolution: %s", runningJob.Resolution))
	}
	if runningJob.VideoCodec != "" {
		codec := runningJob.VideoCodec
		if runningJob.BitDepth > 0 {
			codec = fmt.Sprintf("%s (%d-bit)", codec, runningJob.BitDepth)
		}
		details = append(details, fmt.Sprintf("Source Codec: %s", codec))
	}
	if runningJob.FrameRate != "" {
		details = append(details, fmt.Sprintf("Frame Rate: %s fps", runningJob.FrameRate))
	}
	if runningJob.Container != "" {
		details = append(details, fmt.Sprintf("Container: %s", runningJob.Container))
	}

	// Stream counts
	streamInfo := []string{}
	if runningJob.AudioStreams > 0 {
		streamInfo = append(streamInfo, fmt.Sprintf("%d audio", runningJob.AudioStreams))
	}
	if runningJob.SubStreams > 0 {
		streamInfo = append(streamInfo, fmt.Sprintf("%d subtitle", runningJob.SubStreams))
	}
	if len(streamInfo) > 0 {
		details = append(details, fmt.Sprintf("Streams: %s", strings.Join(streamInfo, ", ")))
	}

	// Sizes - only show real data, no estimates
	details = append(details, fmt.Sprintf("Original Size: %s", formatSize(runningJob.OriginalSize)))
	
	// Show NewSize if available (actual output size during transcoding)
	if runningJob.NewSize > 0 {
		savings := float64(runningJob.OriginalSize-runningJob.NewSize) / float64(runningJob.OriginalSize) * 100
		details = append(details, fmt.Sprintf("Current Size: %s (%.1f%% reduction)",
			formatSize(runningJob.NewSize), savings))
	} else if runningJob.EstimatedSize > 0 {
		// Show rough estimate when we don't have actual size yet
		estSavings := float64(runningJob.OriginalSize-runningJob.EstimatedSize) / float64(runningJob.OriginalSize) * 100
		details = append(details, fmt.Sprintf("Rough Est. Size: %s (~%.1f%% reduction)",
			formatSize(runningJob.EstimatedSize), estSavings))
	}

	// Processing time
	if runningJob.StartedAt != nil {
		elapsed := time.Since(*runningJob.StartedAt)
		details = append(details, fmt.Sprintf("Elapsed Time: %s", formatElapsed(elapsed)))
	}

	// Source classification indicator
	if runningJob.IsWebRipLike {
		details = append(details, "Type: Web-like (web-safe flags enabled)")
	} else {
		details = append(details, "Type: Disc-like (standard flags)")
	}

	return runningStyle.Render(strings.Join(details, "\n")), true
}

// renderQueueSummary builds a quick status snapshot similar to floating widgets in btop.
func renderQueueSummary(jobList []*jobs.Job) string {
	var total, pending, running, success, failed, skipped int

	for _, job := range jobList {
		total++
		switch job.Status {
		case jobs.JobStatusPending:
			pending++
		case jobs.JobStatusRunning:
			running++
		case jobs.JobStatusSuccess:
			success++
		case jobs.JobStatusFailed:
			failed++
		case jobs.JobStatusSkipped:
			skipped++
		}
	}

	lines := []string{
		renderSummaryLine("TOTAL", total),
		renderSummaryLine("PENDING", pending),
		renderSummaryLine("RUNNING", running),
		renderSummaryLine("SUCCESS", success),
		renderSummaryLine("FAILED", failed),
		renderSummaryLine("SKIPPED", skipped),
	}

	return strings.Join(lines, "\n")
}

func renderSummaryLine(label string, value int) string {
	return fmt.Sprintf("%s %s",
		summaryLabelStyle.Render(fmt.Sprintf("%-8s", label)),
		summaryValueStyle.Render(fmt.Sprintf("%4d", value)),
	)
}

// renderJobTable renders the job table.
func renderJobTable(jobs []*jobs.Job, width int, maxLines int) string {
	if len(jobs) == 0 {
		return "No jobs found"
	}

	if maxLines < 2 {
		maxLines = 2
	}

	// Calculate column widths
	colWidths := calculateColumnWidths(width)

	// Header - include EST_SIZE for rough estimates
	header := renderRow(
		[]string{"STATUS", "FILE", "CODEC", "RESOLUTION", "ORIG_SIZE", "EST_SIZE", "NEW_SIZE", "SAVINGS", "DURATION", "REASON"},
		colWidths,
		true,
	)

	// Rows
	var rows []string
	rows = append(rows, headerStyle.Render(header))

	remaining := maxLines - 1
	visibleCount := 0

	for _, job := range jobs {
		if remaining == 0 {
			break
		}

		row := renderJobRow(job, colWidths)
		rows = append(rows, row)
		visibleCount++
		remaining--
	}

	if len(jobs) > visibleCount {
		rows = append(rows, mutedStyle.Render(
			fmt.Sprintf("… %d additional jobs not shown (table truncated to keep layout readable)", len(jobs)-visibleCount),
		))
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

// formatElapsed formats elapsed time duration.
func formatElapsed(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// calculateColumnWidths calculates column widths based on available width.
func calculateColumnWidths(totalWidth int) map[string]int {
	// Fixed widths for columns
	widths := map[string]int{
		"STATUS":     10,
		"CODEC":      8,
		"RESOLUTION": 12,
		"ORIG_SIZE":  11,
		"EST_SIZE":   11,
		"NEW_SIZE":   11,
		"SAVINGS":    9,
		"DURATION":   9,
		"REASON":     40, // Increased to show more error details
	}

	// Calculate FILE column width (remaining space)
	usedWidth := widths["STATUS"] + widths["CODEC"] + widths["RESOLUTION"] +
		widths["ORIG_SIZE"] + widths["EST_SIZE"] + widths["NEW_SIZE"] +
		widths["SAVINGS"] + widths["DURATION"] + widths["REASON"] + 9 // separators
	fileWidth := totalWidth - usedWidth - 2 // padding
	if fileWidth < 15 {
		fileWidth = 15
	}
	widths["FILE"] = fileWidth

	return widths
}

// renderRow renders a table row.
func renderRow(columns []string, widths map[string]int, isHeader bool) string {
	colNames := []string{"STATUS", "FILE", "CODEC", "RESOLUTION", "ORIG_SIZE", "EST_SIZE", "NEW_SIZE", "SAVINGS", "DURATION", "REASON"}
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
	codec := job.SourceCodec
	if codec == "" {
		codec = "-"
	}
	resolution := job.Resolution
	if resolution == "" {
		resolution = "-"
	}
	origSize := formatSize(job.OriginalSize)
	estSize := "-"
	if job.EstimatedSize > 0 {
		estSize = "~" + formatSize(job.EstimatedSize)
	}
	newSize := formatSize(job.NewSize)
	// Only show savings if we have actual NewSize (real data)
	savings := calculateSavings(job.OriginalSize, job.NewSize)
	duration := formatDuration(job)
	reason := job.Reason
	if reason == "" {
		reason = "-"
	}

	row := renderRow(
		[]string{status, fileName, codec, resolution, origSize, estSize, newSize, savings, duration, reason},
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
