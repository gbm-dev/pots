package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors — muted, professional palette
	colorPrimary   = lipgloss.Color("#7D56F4")
	colorSecondary = lipgloss.Color("#6C6C6C")
	colorSuccess   = lipgloss.Color("#73D216")
	colorError     = lipgloss.Color("#FF5555")
	colorWarning   = lipgloss.Color("#F4BF75")
	colorMuted     = lipgloss.Color("#555555")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess)

	warningStyle = lipgloss.NewStyle().
			Foreground(colorWarning)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(1, 2)

	bannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWarning).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorWarning).
			Padding(0, 1)

	labelStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))
)
