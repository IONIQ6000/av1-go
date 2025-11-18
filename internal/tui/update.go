package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/yourname/av1qsvd/internal/jobs"
)

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Refresh):
			return m, tea.Batch(
				refreshJobs(m.jobsDir),
				refreshMetrics(),
			)
		}
		return m, nil

	case jobsMsg:
		// Sort jobs by CreatedAt, newest first
		m.jobs = msg.jobs
		sortJobsByNewest(m.jobs)
		m.lastRefresh = time.Now()
		return m, nil

	case metricsMsg:
		// Update CPU and memory metrics
		cpuPercent, err := cpu.Percent(time.Second, false)
		if err == nil && len(cpuPercent) > 0 {
			m.cpuPercent = cpuPercent[0]
		}

		memInfo, err := mem.VirtualMemory()
		if err == nil {
			m.memPercent = memInfo.UsedPercent
		}

		return m, nil

	case tickMsg:
		// Periodic refresh
		return m, tea.Batch(
			refreshJobs(m.jobsDir),
			refreshMetrics(),
			tick(),
		)

	case errMsg:
		// Log error but continue
		return m, nil
	}

	return m, nil
}

// sortJobsByNewest sorts jobs by CreatedAt, newest first.
func sortJobsByNewest(jobs []*jobs.Job) {
	for i := 0; i < len(jobs)-1; i++ {
		for j := i + 1; j < len(jobs); j++ {
			if jobs[i].CreatedAt.Before(jobs[j].CreatedAt) {
				jobs[i], jobs[j] = jobs[j], jobs[i]
			}
		}
	}
}

// keys defines key bindings.
type keyMap struct {
	Quit    key.Binding
	Refresh key.Binding
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
}

