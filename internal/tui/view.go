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
	// Clean btop-inspired color scheme - subtle and professional
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("250")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("238")).
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("250")).
			Padding(1, 1).
			Margin(0, 1, 1, 0)

	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("252"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))

	// Status colors - subtle
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("76"))

	failedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("160"))

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	pendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	skippedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("136"))

	// Bar colors - subtle
	cpuColor = lipgloss.Color("196")
	memColor = lipgloss.Color("39")
	gpuColor = lipgloss.Color("226")
)

// View renders the TUI.
func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	// Clean title bar
	title := titleStyle.Width(m.width - 2).Render("AV1 Transcoding Daemon")

	// Top row: Metrics and Summary side by side
	metricsWidth := maxInt(40, m.width/2-4)
	if metricsWidth > 50 {
		metricsWidth = 50
	}
	summaryWidth := maxInt(40, m.width-metricsWidth-6)

	metricsPanel := renderMetricsPanel(m.cpuPercent, m.memPercent, m.gpuPercent, metricsWidth)
	summaryPanel := renderSummaryPanel(m.jobs, summaryWidth)
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, metricsPanel, summaryPanel)

	// Active job panel
	activeBody, hasActive := renderActiveJob(m.jobs)
	if !hasActive {
		activeBody = mutedStyle.Render("No active transcoding job")
	}
	activePanel := renderPanel("ACTIVE JOB", activeBody, m.width-4)

	// Job queue table
	tableWidth := maxInt(80, m.width-4)
	titleHeight := lipgloss.Height(title)
	topRowHeight := lipgloss.Height(topRow)
	activeHeight := lipgloss.Height(activePanel)
	statusHeight := 1
	availableBody := m.height - (titleHeight + topRowHeight + activeHeight + statusHeight) - 6
	if availableBody < 5 {
		availableBody = 5
	}

	jobsPanel := renderPanel("JOB QUEUE", renderJobTable(m.jobs, tableWidth, availableBody), m.width-2)

	// Status bar
	statusBar := renderStatusBar(m.jobs, m.jobsDir, m.lastRefresh, m.width-2)

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		topRow,
		activePanel,
		jobsPanel,
		statusBar,
	)
}

// renderPanel wraps content in a clean panel.
func renderPanel(title, body string, width int) string {
	titleBar := panelTitleStyle.Render(" " + title + " ")
	content := titleBar + "\n" + body

	if width > 0 {
		return panelStyle.Width(width).Render(content)
	}
	return panelStyle.Render(content)
}

// renderMetricsPanel renders CPU, memory, and GPU metrics.
func renderMetricsPanel(cpuPercent, memPercent, gpuPercent float64, width int) string {
	lines := []string{
		renderBar("CPU", cpuPercent, cpuColor, width-4),
		renderBar("MEM", memPercent, memColor, width-4),
		renderBar("GPU", gpuPercent, gpuColor, width-4),
	}
	body := strings.Join(lines, "\n")
	return renderPanel("SYSTEM METRICS", body, width)
}

// renderBar renders a clean progress bar.
func renderBar(label string, value float64, color lipgloss.Color, width int) string {
	barWidth := width - 12 // Leave room for label and percentage
	if barWidth < 10 {
		barWidth = 10
	}

	filled := int((value / 100.0) * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}

	// Simple filled bar
	filledBar := strings.Repeat("█", filled)
	emptyBar := strings.Repeat("░", barWidth-filled)
	bar := lipgloss.NewStyle().Foreground(color).Render(filledBar + emptyBar)

	// Percentage color based on value
	var percentColor lipgloss.Color
	if value < 50 {
		percentColor = lipgloss.Color("76") // Green
	} else if value < 80 {
		percentColor = lipgloss.Color("226") // Yellow
	} else {
		percentColor = lipgloss.Color("196") // Red
	}

	percent := lipgloss.NewStyle().Foreground(percentColor).Render(fmt.Sprintf("%5.1f%%", value))
	labelText := labelStyle.Render(fmt.Sprintf("%-3s", label))

	return fmt.Sprintf("%s %s %s", labelText, bar, percent)
}

// renderSummaryPanel renders queue summary.
func renderSummaryPanel(jobList []*jobs.Job, width int) string {
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
		renderSummaryLine("Total", total, lipgloss.Color("250")),
		renderSummaryLine("Pending", pending, lipgloss.Color("244")),
		renderSummaryLine("Running", running, lipgloss.Color("39")),
		renderSummaryLine("Success", success, lipgloss.Color("76")),
		renderSummaryLine("Failed", failed, lipgloss.Color("160")),
		renderSummaryLine("Skipped", skipped, lipgloss.Color("136")),
	}

	body := strings.Join(lines, "\n")
	return renderPanel("QUEUE SUMMARY", body, width)
}

// renderSummaryLine renders a summary line.
func renderSummaryLine(label string, value int, color lipgloss.Color) string {
	labelText := labelStyle.Render(fmt.Sprintf("%-8s", label))
	valueText := lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("%d", value))
	return fmt.Sprintf("%s %s", labelText, valueText)
}

// renderActiveJob renders active job details.
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

	var lines []string

	// File name
	fileName := filepath.Base(runningJob.SourcePath)
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("File:"), valueStyle.Render(fileName)))

	// Technical details
	if runningJob.Resolution != "" {
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Resolution:"), valueStyle.Render(runningJob.Resolution)))
	}
	if runningJob.VideoCodec != "" {
		codec := runningJob.VideoCodec
		if runningJob.BitDepth > 0 {
			codec = fmt.Sprintf("%s (%d-bit)", codec, runningJob.BitDepth)
		}
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Codec:"), valueStyle.Render(codec)))
	}
	if runningJob.FrameRate != "" {
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Frame Rate:"), valueStyle.Render(runningJob.FrameRate+" fps")))
	}

	// Streams
	streamParts := []string{}
	if runningJob.AudioStreams > 0 {
		streamParts = append(streamParts, fmt.Sprintf("%d audio", runningJob.AudioStreams))
	}
	if runningJob.SubStreams > 0 {
		streamParts = append(streamParts, fmt.Sprintf("%d subtitle", runningJob.SubStreams))
	}
	if len(streamParts) > 0 {
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Streams:"), valueStyle.Render(strings.Join(streamParts, ", "))))
	}

	// Sizes
	lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Original:"), valueStyle.Render(formatSize(runningJob.OriginalSize))))

	if runningJob.NewSize > 0 {
		savings := float64(runningJob.OriginalSize-runningJob.NewSize) / float64(runningJob.OriginalSize) * 100
		lines = append(lines, fmt.Sprintf("%s %s (%.1f%% reduction)",
			labelStyle.Render("Current:"),
			valueStyle.Render(formatSize(runningJob.NewSize)),
			savings))
	} else if runningJob.EstimatedSize > 0 {
		estSavings := float64(runningJob.OriginalSize-runningJob.EstimatedSize) / float64(runningJob.OriginalSize) * 100
		lines = append(lines, fmt.Sprintf("%s %s (~%.1f%% reduction)",
			labelStyle.Render("Est. Size:"),
			valueStyle.Render(formatSize(runningJob.EstimatedSize)),
			estSavings))
	}

	// Time
	if runningJob.StartedAt != nil {
		elapsed := time.Since(*runningJob.StartedAt)
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Elapsed:"), valueStyle.Render(formatElapsed(elapsed))))
	}

	// Type
	if runningJob.IsWebRipLike {
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Type:"), valueStyle.Render("Web-like")))
	} else {
		lines = append(lines, fmt.Sprintf("%s %s", labelStyle.Render("Type:"), valueStyle.Render("Disc-like")))
	}

	return strings.Join(lines, "\n"), true
}

// renderJobTable renders the job queue table.
func renderJobTable(jobs []*jobs.Job, width int, maxLines int) string {
	if len(jobs) == 0 {
		return mutedStyle.Render("No jobs in queue")
	}

	if maxLines < 2 {
		maxLines = 2
	}

	colWidths := calculateColumnWidths(width)

	// Header
	header := renderRow(
		[]string{"STATUS", "FILE", "CODEC", "RES", "ORIG", "EST", "NEW", "SAVE", "TIME", "REASON"},
		colWidths,
		true,
	)

	var rows []string
	rows = append(rows, panelTitleStyle.Render(header))

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
			fmt.Sprintf("… %d more jobs", len(jobs)-visibleCount),
		))
	}

	return strings.Join(rows, "\n")
}

// renderRow renders a table row.
func renderRow(columns []string, widths map[string]int, isHeader bool) string {
	colNames := []string{"STATUS", "FILE", "CODEC", "RES", "ORIG", "EST", "NEW", "SAVE", "TIME", "REASON"}
	var parts []string
	for i, colName := range colNames {
		width := widths[colName]
		text := ""
		if i < len(columns) {
			text = columns[i]
		}
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

// renderStatusBar renders the status bar.
func renderStatusBar(jobList []*jobs.Job, jobsDir string, lastRefresh time.Time, width int) string {
	var stats struct {
		total   int
		running int
		failed  int
	}

	for _, job := range jobList {
		stats.total++
		switch job.Status {
		case jobs.JobStatusRunning:
			stats.running++
		case jobs.JobStatusFailed:
			stats.failed++
		}
	}

	runningText := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(fmt.Sprintf("%d", stats.running))
	failedText := lipgloss.NewStyle().Foreground(lipgloss.Color("160")).Render(fmt.Sprintf("%d", stats.failed))

	statusText := fmt.Sprintf("Jobs: %d total | %s running | %s failed | Dir: %s | Updated: %s | [q]uit [r]efresh",
		stats.total,
		runningText,
		failedText,
		jobsDir,
		lastRefresh.Format("15:04:05"),
	)

	if len(statusText) > width {
		statusText = statusText[:width-3] + "..."
	}

	return statusBarStyle.Width(width).Render(statusText)
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

// calculateColumnWidths calculates column widths.
func calculateColumnWidths(totalWidth int) map[string]int {
	widths := map[string]int{
		"STATUS": 8,
		"CODEC":  6,
		"RES":    6,
		"ORIG":   8,
		"EST":    8,
		"NEW":    8,
		"SAVE":   7,
		"TIME":   6,
		"REASON": 30,
	}

	usedWidth := widths["STATUS"] + widths["CODEC"] + widths["RES"] +
		widths["ORIG"] + widths["EST"] + widths["NEW"] +
		widths["SAVE"] + widths["TIME"] + widths["REASON"] + 9
	fileWidth := totalWidth - usedWidth - 2
	if fileWidth < 15 {
		fileWidth = 15
	}
	widths["FILE"] = fileWidth

	return widths
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
