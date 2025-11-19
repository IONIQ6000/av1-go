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
	// btop-inspired color scheme
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("51")). // Bright cyan
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("87")) // Bright cyan

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("66")). // Cyan-gray border
			BorderBackground(lipgloss.Color("235")).
			Background(lipgloss.Color("235")). // Dark background
			Foreground(lipgloss.Color("250")). // Light text
			Padding(1, 2).
			Margin(1, 1, 0, 0)

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("87")). // Bright cyan
			Background(lipgloss.Color("235"))

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238")) // Darker gray

	summaryLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))

	summaryValueStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("82")) // Bright green

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")). // Bright green
			Bold(true)

	failedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")). // Bright red
			Bold(true)

	skippedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")). // Yellow
			Bold(true)

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("87")). // Bright cyan
			Bold(true)

	pendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")) // Gray

	// Bar colors
	cpuBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red for CPU

	memBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("51")) // Cyan for Memory

	gpuBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")) // Yellow for GPU
)

// View renders the TUI.
func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	title := titleStyle.Render("╭─ AV1 Transcoding Daemon ────────────────────────────────────────────╮")

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

// renderPanel wraps content inside a floating panel with a title (btop-style).
func renderPanel(title, body string, width int) string {
	// Add icon/prefix to title
	titlePrefix := "▐ "
	content := panelTitleStyle.Render(titlePrefix + title)
	if body != "" {
		content += "\n" + body
	} else {
		content += "\n" + mutedStyle.Render("  ─")
	}

	if width > 0 {
		return panelStyle.Width(width).Render(content)
	}
	return panelStyle.Render(content)
}

// renderMetrics renders CPU, memory, and GPU usage bars with btop-style colors.
func renderMetrics(cpuPercent, memPercent, gpuPercent float64) string {
	lines := []string{
		renderBar("CPU", cpuPercent, 24, cpuBarStyle),
		renderBar("MEM", memPercent, 24, memBarStyle),
		renderBar("GPU", gpuPercent, 24, gpuBarStyle),
	}
	return strings.Join(lines, "\n")
}

// renderBar renders a progress bar with btop-style gradient colors.
func renderBar(label string, value float64, width int, barStyle lipgloss.Style) string {
	filled := int((value / 100.0) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	
	// Create gradient effect - use different shades based on fill level
	var barParts []string
	for i := 0; i < width; i++ {
		if i < filled {
			// Use gradient: darker at start, brighter at end
			if i < filled/3 {
				barParts = append(barParts, barStyle.Foreground(lipgloss.Color("88")).Render("▁"))
			} else if i < filled*2/3 {
				barParts = append(barParts, barStyle.Foreground(lipgloss.Color("196")).Render("▃"))
			} else {
				barParts = append(barParts, barStyle.Foreground(lipgloss.Color("196")).Render("█"))
			}
		} else {
			barParts = append(barParts, mutedStyle.Render("▁"))
		}
	}
	
	bar := strings.Join(barParts, "")
	
	// Color-code the percentage based on value
	var percentStyle lipgloss.Style
	if value < 50 {
		percentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82")) // Green
	} else if value < 80 {
		percentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // Yellow
	} else {
		percentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	}
	
	percent := percentStyle.Bold(true).Render(fmt.Sprintf("%5.1f%%", value))
	return fmt.Sprintf("%s: %s %s", label, bar, percent)
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

	// Header with icon
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

// renderSummaryLine renders a summary line with btop-style formatting.
func renderSummaryLine(label string, value int) string {
	var valueStyle lipgloss.Style
	switch label {
	case "SUCCESS":
		valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
	case "FAILED":
		valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	case "RUNNING":
		valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("87")).Bold(true)
	case "PENDING":
		valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226")).Bold(true)
	case "SKIPPED":
		valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	default:
		valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Bold(true)
	}
	
	return fmt.Sprintf("%s: %s", 
		summaryLabelStyle.Render(label),
		valueStyle.Render(fmt.Sprintf("%d", value)))
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

// renderStatusBar renders the status bar at the bottom (btop-style).
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

	// Color-code the stats
	runningText := lipgloss.NewStyle().Foreground(lipgloss.Color("87")).Bold(true).Render(fmt.Sprintf("%d", stats.running))
	failedText := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render(fmt.Sprintf("%d", stats.failed))
	
	statusText := fmt.Sprintf("Jobs: %d total | %s running | %s failed | %d skipped | Dir: %s | Last refresh: %s | Press 'q' to quit, 'r' to refresh",
		stats.total,
		runningText,
		failedText,
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
