package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yourname/av1qsvd/internal/config"
	"github.com/yourname/av1qsvd/internal/tui"
)

func main() {
	// Load config to get jobs directory
	// Try to load from /etc/av1qsvd/config.json, fallback to default
	configPath := "/etc/av1qsvd/config.json"
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		// Fallback to default config
		cfg = config.DefaultConfig()
	}

	// Create TUI model
	m := tui.NewModel(cfg.JobStateDir)

	// Create Bubble Tea program
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
