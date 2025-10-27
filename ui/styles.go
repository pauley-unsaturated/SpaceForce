package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette
var (
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary = lipgloss.Color("#06B6D4") // Cyan
	ColorSuccess   = lipgloss.Color("#10B981") // Green
	ColorWarning   = lipgloss.Color("#F59E0B") // Amber
	ColorDanger    = lipgloss.Color("#EF4444") // Red
	ColorMuted     = lipgloss.Color("#6B7280") // Gray
	ColorBorder    = lipgloss.Color("#374151") // Dark gray
	ColorSelected  = lipgloss.Color("#1F2937") // Very dark gray
)

// Styles
var (
	// Title style
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	// Subtitle style
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			MarginBottom(1)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	ActiveBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2)

	// List item styles
	SelectedItemStyle = lipgloss.NewStyle().
				Background(ColorSelected).
				Foreground(ColorPrimary).
				Bold(true)

	NormalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	// Size styles (for displaying file sizes)
	SizeSmallStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	SizeMediumStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	SizeLargeStyle = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	// Safety level styles
	SafeStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	RiskyStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	DangerousStyle = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	// Tab styles
	ActiveTabStyle = lipgloss.NewStyle().
			Background(ColorPrimary).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 2).
			Bold(true)

	InactiveTabStyle = lipgloss.NewStyle().
				Background(ColorBorder).
				Foreground(ColorMuted).
				Padding(0, 2)

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
			Background(ColorBorder).
			Foreground(ColorMuted).
			Padding(0, 1)

	// Help text
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(1)

	// Progress bar
	ProgressBarFilledStyle = lipgloss.NewStyle().
				Background(ColorPrimary)

	ProgressBarEmptyStyle = lipgloss.NewStyle().
				Background(ColorBorder)
)

