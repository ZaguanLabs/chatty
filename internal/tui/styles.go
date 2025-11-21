package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	ColorUser    = lipgloss.Color("#87d7af") // Soft Green
	ColorAI      = lipgloss.Color("#87afff") // Soft Blue
	ColorSystem  = lipgloss.Color("#767676") // Grey
	ColorError   = lipgloss.Color("#ff5f5f") // Soft Red
	ColorHeader  = lipgloss.Color("#bd93f9") // Purple
	ColorBorder  = lipgloss.Color("#444444") // Dark Grey

	// Styles
	styleApp = lipgloss.NewStyle().
			Padding(1, 2)

	styleHeader = lipgloss.NewStyle().
			Foreground(ColorHeader).
			Bold(true).
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder).
			BorderBottom(true)

	styleFooter = lipgloss.NewStyle().
			Foreground(ColorSystem).
			Faint(true)

	styleInput = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorAI).
			Padding(0, 1)

	styleUserLabel = lipgloss.NewStyle().
			Foreground(ColorUser).
			Bold(true).
			MarginRight(1)

	styleAILabel = lipgloss.NewStyle().
			Foreground(ColorAI).
			Bold(true).
			MarginRight(1)

	styleError = lipgloss.NewStyle().
			Foreground(ColorError)
)
