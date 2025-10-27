package util

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	ColorPrimary   = lipgloss.Color("#7C3AED")
	ColorSecondary = lipgloss.Color("#06B6D4")
	ColorSuccess   = lipgloss.Color("#10B981")
	ColorWarning   = lipgloss.Color("#F59E0B")
	ColorDanger    = lipgloss.Color("#EF4444")
	ColorMuted     = lipgloss.Color("#6B7280")
	ColorBorder    = lipgloss.Color("#374151")
	ColorSelected  = lipgloss.Color("#1F2937")

	// Size styles
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

	// Common UI styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			MarginBottom(1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			MarginTop(1)

	NormalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	SelectedItemStyle = lipgloss.NewStyle().
				Background(ColorSelected).
				Foreground(ColorPrimary).
				Bold(true)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)
)

// FormatBytes converts bytes to human-readable format with color coding
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return SizeSmallStyle.Render("< 1 KB")
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	value := float64(bytes) / float64(div)
	units := []string{"KB", "MB", "GB", "TB", "PB"}

	// Color based on size
	var style lipgloss.Style
	if bytes < 1024*1024 { // < 1 MB
		style = SizeSmallStyle
	} else if bytes < 100*1024*1024 { // < 100 MB
		style = SizeMediumStyle
	} else {
		style = SizeLargeStyle
	}

	// Format the size string
	var sizeStr string
	if value < 10 {
		sizeStr = fmt.Sprintf("%.1f %s", value, units[exp])
	} else {
		sizeStr = fmt.Sprintf("%.0f %s", value, units[exp])
	}

	return style.Width(10).Align(lipgloss.Right).Render(sizeStr)
}

// FormatSafetyLevel returns a styled string for a risk level
func FormatSafetyLevel(riskLevel int) string {
	switch riskLevel {
	case 0:
		return SafeStyle.Render("✓ Safe")
	case 1:
		return SafeStyle.Render("⚠ Low Risk")
	case 2:
		return RiskyStyle.Render("⚠ Review")
	case 3:
		return DangerousStyle.Render("✗ Protected")
	default:
		return "Unknown"
	}
}
