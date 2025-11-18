package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yourname/av1qsvd/internal/jobs"
)

// Model represents the TUI state.
type Model struct {
	jobsDir      string
	jobs         []*jobs.Job
	cpuPercent   float64
	memPercent   float64
	gpuPercent   float64
	width        int
	height       int
	lastRefresh  time.Time
}

// NewModel creates a new TUI model.
func NewModel(jobsDir string) Model {
	return Model{
		jobsDir:     jobsDir,
		jobs:        []*jobs.Job{},
		cpuPercent:  0.0,
		memPercent:  0.0,
		gpuPercent:  0.0,
		lastRefresh: time.Now(),
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		refreshJobs(m.jobsDir),
		refreshMetrics(),
		tick(),
	)
}

// tickCmd sends a tick message every second.
func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type tickMsg time.Time

// refreshJobsCmd loads jobs from disk.
func refreshJobs(jobsDir string) tea.Cmd {
	return func() tea.Msg {
		jobs, err := jobs.LoadAllJobs(jobsDir)
		if err != nil {
			return errMsg{err}
		}
		return jobsMsg{jobs}
	}
}

type jobsMsg struct {
	jobs []*jobs.Job
}

// refreshMetricsCmd loads system metrics.
func refreshMetrics() tea.Cmd {
	return func() tea.Msg {
		return metricsMsg{}
	}
}

type metricsMsg struct{}

type errMsg struct {
	err error
}
