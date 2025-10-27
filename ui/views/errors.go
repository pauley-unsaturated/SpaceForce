package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"spaceforce/util"
)

// ErrorsView displays scan errors and warnings
type ErrorsView struct {
	errors        []error
	selectedIndex int
	height        int
}

// NewErrorsView creates a new errors view
func NewErrorsView(errors []error) *ErrorsView {
	return &ErrorsView{
		errors: errors,
		height: 20,
	}
}

// Init initializes the view
func (ev *ErrorsView) Init() tea.Cmd {
	return nil
}

// Update handles updates
func (ev *ErrorsView) Update(msg tea.Msg) (*ErrorsView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if ev.selectedIndex > 0 {
				ev.selectedIndex--
			}
		case "down", "j":
			if ev.selectedIndex < len(ev.errors)-1 {
				ev.selectedIndex++
			}
		}
	}
	return ev, nil
}

// View renders the view
func (ev *ErrorsView) View() string {
	var b strings.Builder

	if len(ev.errors) == 0 {
		b.WriteString(util.TitleStyle.Render("✓ No Errors"))
		b.WriteString("\n\n")
		b.WriteString(util.HelpStyle.Render("The filesystem scan completed without any errors."))
		return b.String()
	}

	b.WriteString(util.TitleStyle.Render(fmt.Sprintf("⚠ Scan Errors (%d)", len(ev.errors))))
	b.WriteString("\n")
	b.WriteString(util.SubtitleStyle.Render("These directories/files could not be accessed"))
	b.WriteString("\n\n")

	// Reserve lines for title (2), subtitle (3), footer (2)
	// Total chrome: 5 lines + 2 for optional footer = 7 lines worst case
	contentHeight := ev.height - 7
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Calculate viewport
	start := ev.selectedIndex - contentHeight/2
	if start < 0 {
		start = 0
	}
	end := start + contentHeight
	if end > len(ev.errors) {
		end = len(ev.errors)
		start = end - contentHeight
		if start < 0 {
			start = 0
		}
	}

	// Render errors
	for i := start; i < end && i < len(ev.errors); i++ {
		line := ev.renderError(i, i == ev.selectedIndex)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Footer
	if len(ev.errors) > contentHeight {
		b.WriteString("\n")
		b.WriteString(util.HelpStyle.Render(fmt.Sprintf("Showing %d-%d of %d errors",
			start+1, end, len(ev.errors))))
	}

	return b.String()
}

// renderError renders a single error
func (ev *ErrorsView) renderError(index int, selected bool) string {
	err := ev.errors[index]
	errStr := err.Error()

	// Truncate if too long
	maxLen := 90
	if len(errStr) > maxLen {
		errStr = errStr[:maxLen-3] + "..."
	}

	// Format with index
	line := fmt.Sprintf("%3d. %s", index+1, errStr)

	if selected {
		return util.SelectedItemStyle.Render(line)
	}

	// Color based on error type
	lowerErr := strings.ToLower(errStr)
	if strings.Contains(lowerErr, "permission denied") {
		return util.RiskyStyle.Render(line)
	} else if strings.Contains(lowerErr, "not found") || strings.Contains(lowerErr, "no such") {
		return util.SizeSmallStyle.Render(line)
	} else {
		return util.NormalItemStyle.Render(line)
	}
}

// SetHeight sets the viewport height
func (ev *ErrorsView) SetHeight(height int) {
	ev.height = height
}

// GetErrorCount returns the number of errors
func (ev *ErrorsView) GetErrorCount() int {
	return len(ev.errors)
}

// GetErrorsByType returns errors grouped by type
func (ev *ErrorsView) GetErrorsByType() map[string][]error {
	byType := make(map[string][]error)

	for _, err := range ev.errors {
		errStr := strings.ToLower(err.Error())

		if strings.Contains(errStr, "permission denied") {
			byType["Permission Denied"] = append(byType["Permission Denied"], err)
		} else if strings.Contains(errStr, "not found") || strings.Contains(errStr, "no such") {
			byType["Not Found"] = append(byType["Not Found"], err)
		} else if strings.Contains(errStr, "cannot read") {
			byType["Read Error"] = append(byType["Read Error"], err)
		} else {
			byType["Other"] = append(byType["Other"], err)
		}
	}

	return byType
}
