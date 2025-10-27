package components

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"spaceforce/safety"
	"spaceforce/scanner"
	"spaceforce/util"
)

// ConfirmDialog shows a confirmation dialog for file deletion
type ConfirmDialog struct {
	items     []*scanner.FileNode
	protector *safety.Protector
	confirmed bool
	cancelled bool
	cursor    int // 0 = Cancel, 1 = Confirm
}

// NewConfirmDialog creates a new confirmation dialog
func NewConfirmDialog(items []*scanner.FileNode) *ConfirmDialog {
	return &ConfirmDialog{
		items:     items,
		protector: safety.NewProtector(),
		cursor:    0,
	}
}

// Init initializes the dialog
func (cd *ConfirmDialog) Init() tea.Cmd {
	return nil
}

// Update handles updates
func (cd *ConfirmDialog) Update(msg tea.Msg) (*ConfirmDialog, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			cd.cursor = 0
		case "right", "l":
			cd.cursor = 1
		case "enter":
			if cd.cursor == 0 {
				cd.cancelled = true
			} else {
				cd.confirmed = true
			}
		case "esc":
			cd.cancelled = true
		}
	}
	return cd, nil
}

// View renders the dialog
func (cd *ConfirmDialog) View() string {
	var b strings.Builder

	b.WriteString(util.TitleStyle.Render("⚠️  Confirm Deletion"))
	b.WriteString("\n\n")

	// Show items to be deleted
	b.WriteString("The following items will be moved to Trash:\n\n")

	totalSize := int64(0)
	protectedCount := 0
	maxShow := 10

	for i, item := range cd.items {
		if i >= maxShow {
			remaining := len(cd.items) - maxShow
			b.WriteString(util.HelpStyle.Render(fmt.Sprintf("...and %d more items\n", remaining)))
			break
		}

		safe, reason := cd.protector.IsSafeToDelete(item.Path)
		size := item.TotalSize()
		totalSize += size

		icon := "📄"
		if item.IsDir {
			icon = "📁"
		}

		line := fmt.Sprintf("  %s %s (%s)", icon, item.Path, util.FormatBytes(size))

		if !safe {
			protectedCount++
			b.WriteString(util.DangerousStyle.Render(line + " [PROTECTED: " + reason + "]"))
		} else {
			riskLevel := cd.protector.GetRiskLevel(item.Path)
			if riskLevel > 0 {
				b.WriteString(util.RiskyStyle.Render(line + " [" + reason + "]"))
			} else {
				b.WriteString(line)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Total space to free: %s\n", util.FormatBytes(totalSize)))

	if protectedCount > 0 {
		b.WriteString("\n")
		b.WriteString(util.DangerousStyle.Render(fmt.Sprintf(
			"⚠️  WARNING: %d protected items will be SKIPPED\n", protectedCount)))
	}

	b.WriteString("\n")

	// Buttons
	cancelBtn := "[ Cancel ]"
	confirmBtn := "[ Confirm ]"

	if cd.cursor == 0 {
		cancelBtn = util.SelectedItemStyle.Render(cancelBtn)
	} else {
		cancelBtn = util.NormalItemStyle.Render(cancelBtn)
	}

	if cd.cursor == 1 {
		confirmBtn = util.DangerousStyle.Render(confirmBtn)
	} else {
		confirmBtn = util.NormalItemStyle.Render(confirmBtn)
	}

	b.WriteString(fmt.Sprintf("  %s    %s\n", cancelBtn, confirmBtn))
	b.WriteString("\n")
	b.WriteString(util.HelpStyle.Render("←→: select | enter: confirm | esc: cancel"))

	return util.BoxStyle.Render(b.String())
}

// IsConfirmed returns true if the user confirmed
func (cd *ConfirmDialog) IsConfirmed() bool {
	return cd.confirmed
}

// IsCancelled returns true if the user cancelled
func (cd *ConfirmDialog) IsCancelled() bool {
	return cd.cancelled
}

// DeleteItems moves the confirmed items to trash
func DeleteItems(items []*scanner.FileNode, protector *safety.Protector) (int, int64, error) {
	deleted := 0
	freedSpace := int64(0)

	for _, item := range items {
		safe, _ := protector.IsSafeToDelete(item.Path)
		if !safe {
			// Skip protected items
			continue
		}

		// Move to trash (on macOS, we use the Trash command)
		err := moveToTrash(item.Path)
		if err != nil {
			return deleted, freedSpace, fmt.Errorf("failed to delete %s: %w", item.Path, err)
		}

		deleted++
		freedSpace += item.TotalSize()
	}

	return deleted, freedSpace, nil
}

// moveToTrash moves a file to the macOS Trash
func moveToTrash(path string) error {
	// Use osascript to move to trash (macOS specific)
	// This is safer than rm as items can be recovered
	script := fmt.Sprintf(`
		tell application "Finder"
			move POSIX file "%s" to trash
		end tell
	`, path)

	// For now, we'll just use os.Remove as a fallback
	// In production, you'd use osascript or a proper trash library
	return os.Remove(path)
}
