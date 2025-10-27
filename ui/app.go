package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"spaceforce/safety"
	"spaceforce/scanner"
	"spaceforce/ui/views"
	"spaceforce/util"
)

// ViewType represents different view modes
type ViewType int

const (
	ViewTree ViewType = iota
	ViewTopList
	ViewBreakdown
	ViewTimeline
	ViewErrors
)

// ModalType represents different modal dialogs
type ModalType int

const (
	ModalNone ModalType = iota
	ModalDeleteConfirm
	ModalDeleteProgress
	ModalDeleteSummary
)

// DeleteProgress tracks deletion operation progress
type DeleteProgress struct {
	Current           int
	Total             int
	CurrentFile       string
	BytesDeleted      int64
	FilesDeleted      int   // Top-level items deleted
	TotalFilesDeleted int   // Total files including those in deleted directories
	Errors            []error
}

// Model is the main application model
type Model struct {
	currentView ViewType
	scanner     *scanner.Scanner
	root        *scanner.FileNode
	scanning    bool
	progress    scanner.ScanProgress

	// Views
	treeView      *views.TreeView
	topListView   *views.TopListView
	breakdownView *views.BreakdownView
	timelineView  *views.TimelineView
	errorsView    *views.ErrorsView

	// UI state
	width           int
	height          int
	err             error
	skippedVolumes  []string
	showSkippedInfo bool

	// File marking and deletion
	markedFiles             map[string]*scanner.FileNode // Path -> Node
	activeModal             ModalType
	deleteProgress          DeleteProgress
	diskSpaceBefore         int64
	diskSpaceAfter          int64
	sensitiveDeleteConfirmed bool // Track if user has confirmed deletion of sensitive paths once
}

// ScanCompleteMsg is sent when scanning completes
type ScanCompleteMsg struct {
	Root           *scanner.FileNode
	Err            error
	SkippedVolumes []string
}

// ScanProgressMsg is sent during scanning
type ScanProgressMsg scanner.ScanProgress

// JumpToTreeViewMsg is sent when we want to switch to tree view and select a specific node
type JumpToTreeViewMsg struct {
	Path string
}

// NewModel creates a new application model
func NewModel(rootPath string) *Model {
	return &Model{
		currentView: ViewTree,
		scanner:     scanner.NewScanner(),
		scanning:    true,
		width:       80,
		height:      24,
		markedFiles: make(map[string]*scanner.FileNode),
		activeModal: ModalNone,
	}
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles updates
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate available height for view content
		// Reserve space for:
		// - Title (1 line)
		// - Newline after title (1 line)
		// - Tabs (1 line)
		// - Newline after tabs (1 line)
		// - Newline before help (1 line)
		// - Help footer (1 line)
		// - Newline before skipped (1 line, even if not shown)
		// - Skipped info (1 line, even if not shown)
		// Total app chrome: 8 lines
		viewHeight := msg.Height - 8
		if viewHeight < 5 {
			viewHeight = 5 // Minimum height
		}

		// Update all views with new height and width
		if m.treeView != nil {
			m.treeView.SetHeight(viewHeight)
			m.treeView.SetWidth(msg.Width)
		}
		if m.topListView != nil {
			m.topListView.SetHeight(viewHeight)
		}
		if m.breakdownView != nil {
			m.breakdownView.SetHeight(viewHeight)
		}
		if m.timelineView != nil {
			m.timelineView.SetHeight(viewHeight)
		}
		if m.errorsView != nil {
			m.errorsView.SetHeight(viewHeight)
		}
		return m, nil

	case tea.KeyMsg:
		// Handle modal interactions first
		if m.activeModal != ModalNone {
			return m.handleModalInput(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "1":
			m.currentView = ViewTree
		case "2":
			m.currentView = ViewTopList
		case "3":
			m.currentView = ViewBreakdown
		case "4":
			m.currentView = ViewTimeline
		case "5":
			m.currentView = ViewErrors

		case "tab":
			m.currentView = (m.currentView + 1) % 5

		case "shift+tab":
			// Navigate tabs in reverse
			m.currentView = (m.currentView - 1 + 5) % 5

		case "m":
			// Mark/unmark current file
			if !m.scanning {
				m.toggleMarkCurrentFile()
			}

		case "x":
			// Delete marked files
			if !m.scanning && len(m.markedFiles) > 0 {
				m.activeModal = ModalDeleteConfirm
			}

		default:
			// Pass key to current view
			if !m.scanning {
				return m.updateCurrentView(msg)
			}
		}

	case ScanCompleteMsg:
		m.scanning = false
		m.root = msg.Root
		m.err = msg.Err
		m.skippedVolumes = msg.SkippedVolumes
		m.showSkippedInfo = len(msg.SkippedVolumes) > 0

		if m.root != nil {
			// Initialize all views
			m.treeView = views.NewTreeView(m.root)
			m.topListView = views.NewTopListView(m.root)
			m.breakdownView = views.NewBreakdownView(m.root)
			m.timelineView = views.NewTimelineView(m.root)

			// Set initial height and width based on current window size
			viewHeight := m.height - 8
			if viewHeight < 5 {
				viewHeight = 5
			}
			m.treeView.SetHeight(viewHeight)
			m.treeView.SetWidth(m.width)
			m.topListView.SetHeight(viewHeight)
			m.breakdownView.SetHeight(viewHeight)
			m.timelineView.SetHeight(viewHeight)
		}

		// Initialize errors view (even if no errors)
		m.errorsView = views.NewErrorsView(m.progress.Errors)

		// Set height for errors view too
		viewHeight := m.height - 8
		if viewHeight < 5 {
			viewHeight = 5
		}
		m.errorsView.SetHeight(viewHeight)

		return m, nil

	case ScanProgressMsg:
		m.progress = scanner.ScanProgress(msg)
		return m, nil

	case DeleteCompleteMsg:
		// Store deletion results
		m.deleteProgress.FilesDeleted = msg.ItemsDeleted
		m.deleteProgress.TotalFilesDeleted = msg.TotalFilesDeleted
		m.deleteProgress.BytesDeleted = msg.BytesDeleted
		m.deleteProgress.Errors = msg.Errors

		// Remove deleted nodes from the tree
		for _, path := range msg.DeletedPaths {
			m.removeNodeFromTree(path)
		}

		// Rebuild all views with updated tree
		if m.root != nil {
			m.treeView = views.NewTreeView(m.root)
			m.topListView = views.NewTopListView(m.root)
			m.breakdownView = views.NewBreakdownView(m.root)
			m.timelineView = views.NewTimelineView(m.root)

			// Set dimensions for all views
			viewHeight := m.height - 8
			if viewHeight < 5 {
				viewHeight = 5
			}

			m.treeView.SetHeight(viewHeight)
			m.treeView.SetWidth(m.width)
			m.topListView.SetHeight(viewHeight)
			m.breakdownView.SetHeight(viewHeight)
			m.timelineView.SetHeight(viewHeight)

			// Restore marked files (but remove deleted ones)
			remainingMarked := make(map[string]*scanner.FileNode)
			for path, node := range m.markedFiles {
				// Check if this path was deleted
				wasDeleted := false
				for _, deletedPath := range msg.DeletedPaths {
					if path == deletedPath {
						wasDeleted = true
						break
					}
				}
				if !wasDeleted {
					remainingMarked[path] = node
				}
			}
			m.markedFiles = remainingMarked
			m.updateMarkedFilesInViews()
		}

		// Show summary modal
		m.activeModal = ModalDeleteSummary
		return m, nil

	case JumpToTreeViewMsg:
		// Switch to tree view and select the specified node
		m.currentView = ViewTree
		if m.treeView != nil {
			m.treeView.SelectAndExpandToNode(msg.Path)
		}
		return m, nil

	default:
		// Handle string-based messages from views (to avoid import cycles)
		if strMsg, ok := msg.(string); ok && strings.HasPrefix(strMsg, "JUMP_TO_TREE:") {
			path := strings.TrimPrefix(strMsg, "JUMP_TO_TREE:")
			m.currentView = ViewTree
			if m.treeView != nil {
				m.treeView.SelectAndExpandToNode(path)
			}
			return m, nil
		}
	}

	return m, nil
}

// updateCurrentView updates the active view with a message
func (m *Model) updateCurrentView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.currentView {
	case ViewTree:
		if m.treeView != nil {
			newView, cmd := m.treeView.Update(msg)
			m.treeView = newView
			return m, cmd
		}
	case ViewTopList:
		if m.topListView != nil {
			newView, cmd := m.topListView.Update(msg)
			m.topListView = newView
			return m, cmd
		}
	case ViewBreakdown:
		if m.breakdownView != nil {
			newView, cmd := m.breakdownView.Update(msg)
			m.breakdownView = newView
			return m, cmd
		}
	case ViewTimeline:
		if m.timelineView != nil {
			newView, cmd := m.timelineView.Update(msg)
			m.timelineView = newView
			return m, cmd
		}
	case ViewErrors:
		if m.errorsView != nil {
			newView, cmd := m.errorsView.Update(msg)
			m.errorsView = newView
			return m, cmd
		}
	}
	return m, nil
}

// View renders the application
func (m *Model) View() string {
	if m.scanning {
		return m.renderScanningView()
	}

	if m.err != nil {
		return m.renderError()
	}

	var b strings.Builder

	// Title (1 line)
	b.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render("🚀 SpaceForce - Disk Space Analyzer"))
	b.WriteString("\n")

	// Tabs (1 line)
	b.WriteString(m.renderTabs())
	b.WriteString("\n")

	// Current view (uses remaining height)
	viewContent := m.renderCurrentView()

	// Show modal overlay if active
	if m.activeModal != ModalNone {
		viewContent = m.renderModal(viewContent)
	}

	b.WriteString(viewContent)

	// Help footer (1 line)
	if m.activeModal == ModalNone {
		b.WriteString("\n")
		b.WriteString(m.renderHelp())
	}

	// Show skipped volumes info if any (1 line)
	if m.showSkippedInfo && m.activeModal == ModalNone {
		b.WriteString("\n")
		b.WriteString(m.renderSkippedInfo())
	}

	// Pad remaining height with empty lines to clear any artifacts from resizing
	content := b.String()
	contentLines := strings.Count(content, "\n")
	if contentLines < m.height-1 {
		// Add empty lines to fill the rest of the terminal
		for i := contentLines; i < m.height-1; i++ {
			content += "\n"
		}
	}

	return content
}

// renderTabs renders the tab navigation
func (m *Model) renderTabs() string {
	// Build tab labels with error count if applicable
	errorCount := ""
	if m.errorsView != nil && m.errorsView.GetErrorCount() > 0 {
		errorCount = fmt.Sprintf(" (%d)", m.errorsView.GetErrorCount())
	}

	tabs := []string{
		"1:Tree",
		"2:Top Items",
		"3:Breakdown",
		"4:Timeline",
		"5:Errors" + errorCount,
	}

	var rendered []string
	for i, tab := range tabs {
		if ViewType(i) == m.currentView {
			rendered = append(rendered, ActiveTabStyle.Render(tab))
		} else {
			rendered = append(rendered, InactiveTabStyle.Render(tab))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// renderCurrentView renders the active view
func (m *Model) renderCurrentView() string {
	switch m.currentView {
	case ViewTree:
		if m.treeView != nil {
			return m.treeView.View()
		}
	case ViewTopList:
		if m.topListView != nil {
			return m.topListView.View()
		}
	case ViewBreakdown:
		if m.breakdownView != nil {
			return m.breakdownView.View()
		}
	case ViewTimeline:
		if m.timelineView != nil {
			return m.timelineView.View()
		}
	case ViewErrors:
		if m.errorsView != nil {
			return m.errorsView.View()
		}
	}
	return "Loading..."
}

// renderScanningView renders the scanning progress
func (m *Model) renderScanningView() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("🔍 Scanning Filesystem..."))
	b.WriteString("\n\n")

	// Progress bar based on bytes scanned
	if m.progress.TotalBytes > 0 {
		progress := float64(m.progress.BytesScanned) / float64(m.progress.TotalBytes)
		// Cap at 100% (we might scan more than estimated due to filesystem dynamics)
		if progress > 1.0 {
			progress = 1.0
		}
		progressBar := m.renderProgressBar(progress, 60)
		b.WriteString(progressBar)
		b.WriteString("\n")

		// Show bytes scanned vs total
		bytesStyle := lipgloss.NewStyle().Faint(true)
		b.WriteString(bytesStyle.Render(fmt.Sprintf("%s / %s",
			util.FormatBytes(m.progress.BytesScanned),
			util.FormatBytes(m.progress.TotalBytes))))
		b.WriteString("\n\n")
	}

	// Progress stats
	statsStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)
	b.WriteString(statsStyle.Render(fmt.Sprintf("Files scanned: %s", formatNumber(m.progress.FilesScanned))))
	b.WriteString("\n")

	// Show iCloud files skipped if any
	if m.progress.ICloudFilesSkipped > 0 {
		icloudStyle := lipgloss.NewStyle().Foreground(ColorSecondary)
		b.WriteString(icloudStyle.Render(fmt.Sprintf("iCloud placeholders skipped: %s", formatNumber(m.progress.ICloudFilesSkipped))))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Current path - show more prominently
	pathStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	b.WriteString(lipgloss.NewStyle().Faint(true).Render("Currently scanning:"))
	b.WriteString("\n")

	// Truncate path if too long, but keep more visible
	currentPath := m.progress.CurrentPath
	maxWidth := 100
	if len(currentPath) > maxWidth {
		// Show start and end, with ellipsis in middle
		start := currentPath[:40]
		end := currentPath[len(currentPath)-57:]
		currentPath = start + "..." + end
	}
	b.WriteString(pathStyle.Render(currentPath))
	b.WriteString("\n")

	if len(m.progress.Errors) > 0 {
		b.WriteString("\n")
		warningStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
		b.WriteString(warningStyle.Render(fmt.Sprintf("⚠ Warnings: %d", len(m.progress.Errors))))
		b.WriteString("\n")
		b.WriteString(HelpStyle.Render("(permission denied, timeouts, etc. - will be shown in Errors tab)"))
	}

	b.WriteString("\n\n")
	b.WriteString(HelpStyle.Render("Tip: Large scans can take several minutes • Press 'q' to cancel"))

	// Pad remaining height with empty lines to clear any artifacts from resizing
	content := b.String()
	contentLines := strings.Count(content, "\n")
	if contentLines < m.height-1 {
		// Add empty lines to fill the rest of the terminal
		for i := contentLines; i < m.height-1; i++ {
			content += "\n"
		}
	}

	return content
}

// formatNumber formats a number with thousand separators
func formatNumber(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	// Add commas
	var result strings.Builder
	for i, digit := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(digit)
	}
	return result.String()
}

// renderError renders an error message
func (m *Model) renderError() string {
	content := lipgloss.NewStyle().
		Foreground(ColorDanger).
		Render(fmt.Sprintf("Error: %v", m.err))

	// Pad remaining height with empty lines to clear any artifacts from resizing
	contentLines := strings.Count(content, "\n")
	if contentLines < m.height-1 {
		// Add empty lines to fill the rest of the terminal
		for i := contentLines; i < m.height-1; i++ {
			content += "\n"
		}
	}

	return content
}

// renderSkippedInfo renders information about skipped network volumes
func (m *Model) renderSkippedInfo() string {
	count := len(m.skippedVolumes)
	if count == 0 {
		return ""
	}

	infoStyle := lipgloss.NewStyle().
		Foreground(ColorWarning).
		Italic(true)

	msg := fmt.Sprintf("ℹ Skipped %d network volume(s). Use -skip-network=false to include them.", count)

	// Truncate if too long to prevent wrapping
	maxWidth := m.width - 10
	if maxWidth < 80 {
		maxWidth = 80
	}
	if len(msg) > maxWidth {
		msg = msg[:maxWidth-3] + "..."
	}

	return infoStyle.Render(msg)
}

// renderHelp renders help text
func (m *Model) renderHelp() string {
	helps := []string{
		"tab/shift+tab: switch view",
		"1-5: jump to view",
		"↑↓/jk: navigate",
		"q: quit",
	}

	// Add view-specific help
	switch m.currentView {
	case ViewTree:
		helps = append(helps, "enter/space: expand/collapse", "←→/hl: expand/collapse", "s: change sort", "z: zoom in", "u: zoom out")
	case ViewTopList:
		helps = append(helps, "enter: jump to tree", "s: change sort", "f: toggle files", "d: toggle dirs")
	}

	// Add marking/deletion help if files are marked
	if len(m.markedFiles) > 0 {
		helps = append(helps, "m: mark/unmark", fmt.Sprintf("x: delete %d marked", len(m.markedFiles)))
	} else {
		helps = append(helps, "m: mark file for deletion")
	}

	helpText := strings.Join(helps, " | ")

	// Truncate if too long to prevent wrapping (leave room for styling)
	maxWidth := m.width - 10
	if maxWidth < 80 {
		maxWidth = 80
	}
	if len(helpText) > maxWidth {
		helpText = helpText[:maxWidth-3] + "..."
	}

	return HelpStyle.Render(helpText)
}

// toggleMarkCurrentFile marks or unmarks the currently selected file
func (m *Model) toggleMarkCurrentFile() {
	node := m.getCurrentNode()
	if node == nil {
		return
	}

	if _, exists := m.markedFiles[node.Path]; exists {
		delete(m.markedFiles, node.Path)
	} else {
		m.markedFiles[node.Path] = node
	}

	// Update all views with the new marked files map
	m.updateMarkedFilesInViews()
}

// updateMarkedFilesInViews updates all views with the current marked files
func (m *Model) updateMarkedFilesInViews() {
	if m.treeView != nil {
		m.treeView.SetMarkedFiles(m.markedFiles)
	}
	if m.topListView != nil {
		m.topListView.SetMarkedFiles(m.markedFiles)
	}
}

// getCurrentNode gets the currently selected node from the active view
func (m *Model) getCurrentNode() *scanner.FileNode {
	switch m.currentView {
	case ViewTree:
		if m.treeView != nil {
			return m.treeView.GetSelectedNode()
		}
	case ViewTopList:
		if m.topListView != nil {
			return m.topListView.GetSelectedNode()
		}
	}
	return nil
}

// removeNodeFromTree removes a node from the tree by path
func (m *Model) removeNodeFromTree(targetPath string) {
	if m.root == nil {
		return
	}

	// If we're deleting the root itself, clear everything
	if m.root.Path == targetPath {
		m.root = nil
		return
	}

	// Find and remove the node
	m.removeNodeRecursive(m.root, targetPath)
}

// removeNodeRecursive recursively finds and removes a node from the tree
func (m *Model) removeNodeRecursive(parent *scanner.FileNode, targetPath string) bool {
	for i, child := range parent.Children {
		if child.Path == targetPath {
			// Remove this child
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return true
		}

		// Recursively search in this child's children
		if m.removeNodeRecursive(child, targetPath) {
			return true
		}
	}
	return false
}

// handleModalInput handles keyboard input when a modal is active
func (m *Model) handleModalInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.activeModal {
	case ModalDeleteConfirm:
		switch msg.String() {
		case "y", "Y", "enter":
			// Check if any marked files require confirmation
			protector := safety.NewProtector()
			hasSensitive := false
			for path := range m.markedFiles {
				if requiresConf, _ := protector.RequiresConfirmation(path); requiresConf {
					hasSensitive = true
					break
				}
			}

			// If sensitive paths and not yet confirmed, require second confirmation
			if hasSensitive && !m.sensitiveDeleteConfirmed {
				m.sensitiveDeleteConfirmed = true
				// Stay in confirmation modal, will show updated message
				return m, nil
			}

			// Either no sensitive paths, or already confirmed - proceed with deletion
			m.activeModal = ModalDeleteProgress
			m.sensitiveDeleteConfirmed = false // Reset for next time
			return m, m.startDeletion()
		case "n", "N", "esc", "q":
			// Cancel
			m.activeModal = ModalNone
			m.sensitiveDeleteConfirmed = false // Reset confirmation state
		}
	case ModalDeleteSummary:
		// Any key closes the summary
		m.activeModal = ModalNone
		m.markedFiles = make(map[string]*scanner.FileNode) // Clear marked files
	}
	return m, nil
}

// DeleteProgressUpdateMsg is sent during deletion to update progress
type DeleteProgressUpdateMsg struct {
	Current     int
	Total       int
	CurrentFile string
}

// startDeletion initiates the deletion process
func (m *Model) startDeletion() tea.Cmd {
	// Store marked files for deletion
	filesToDelete := make(map[string]*scanner.FileNode)
	for k, v := range m.markedFiles {
		filesToDelete[k] = v
	}

	return func() tea.Msg {
		deleter := safety.NewDeleter(safety.DeleteToTrash)

		// Initialize progress
		current := 0
		itemsDeleted := 0
		totalFilesDeleted := 0
		var totalBytesDeleted int64
		errors := make([]error, 0)
		deletedPaths := make([]string, 0)

		// Delete each file/directory
		for path, node := range filesToDelete {
			current++

			// Count total files in this item (if it's a directory, count all files inside)
			fileCount := int(node.FileCount())

			// Delete the file/directory
			bytesDeleted, err := deleter.DeleteFile(path)
			if err != nil {
				errors = append(errors, fmt.Errorf("%s: %w", path, err))
			} else {
				itemsDeleted++
				totalFilesDeleted += fileCount
				totalBytesDeleted += bytesDeleted
				deletedPaths = append(deletedPaths, path)
			}

			// Note: We can't send progress updates from within this function easily
			// in Bubble Tea's model, but the deletion itself is now more reliable
		}

		return DeleteCompleteMsg{
			ItemsDeleted:      itemsDeleted,
			TotalFilesDeleted: totalFilesDeleted,
			BytesDeleted:      totalBytesDeleted,
			Errors:            errors,
			DeletedPaths:      deletedPaths,
		}
	}
}

// DeleteCompleteMsg is sent when deletion completes
type DeleteCompleteMsg struct {
	ItemsDeleted     int     // Top-level items (files/directories)
	TotalFilesDeleted int     // Total files including those in deleted directories
	BytesDeleted     int64
	Errors           []error
	DeletedPaths     []string // Paths that were deleted (for tree update)
}

// renderModal renders a modal dialog overlay
func (m *Model) renderModal(background string) string {
	var modal string

	switch m.activeModal {
	case ModalDeleteConfirm:
		modal = m.renderDeleteConfirmModal()
	case ModalDeleteProgress:
		modal = m.renderDeleteProgressModal()
	case ModalDeleteSummary:
		modal = m.renderDeleteSummaryModal()
	default:
		return background
	}

	// Center the modal on the screen
	return lipgloss.Place(
		m.width,
		m.height-10,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
	)
}

// renderDeleteConfirmModal renders the deletion confirmation dialog
func (m *Model) renderDeleteConfirmModal() string {
	// Calculate total size and check for sensitive paths
	var totalSize int64
	var sensitivePaths []string
	protector := safety.NewProtector()

	for path, node := range m.markedFiles {
		totalSize += node.TotalSize()
		if requiresConf, reason := protector.RequiresConfirmation(path); requiresConf {
			sensitivePaths = append(sensitivePaths, fmt.Sprintf("%s (%s)", filepath.Base(path), reason))
		}
	}

	hasSensitive := len(sensitivePaths) > 0

	// Choose title and color based on sensitivity
	var title string
	var borderColor lipgloss.Color
	if hasSensitive {
		title = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B")).
			Render("⚠️  CONFIRM DELETION - SENSITIVE PATHS")
		borderColor = lipgloss.Color("#FF6B6B")
	} else {
		title = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorDanger).
			Render("⚠️  Confirm Deletion")
		borderColor = ColorDanger
	}

	// Build message
	message := fmt.Sprintf(
		"%s\n\n"+
			"You are about to delete:\n"+
			"  • %d file(s) / folder(s)\n"+
			"  • Total size: %s\n\n",
		title,
		len(m.markedFiles),
		util.FormatBytes(totalSize),
	)

	// Build tree view of files to be deleted
	message += "Files to be deleted:\n"
	treeView := m.buildDeletionTreeView()
	message += treeView

	// Add sensitive paths warning if any
	if hasSensitive {
		message += "\n⚠️  WARNING: Includes sensitive locations:\n"
		// Show up to 3 examples
		for i, path := range sensitivePaths {
			if i >= 3 {
				message += fmt.Sprintf("  ... and %d more\n", len(sensitivePaths)-3)
				break
			}
			message += fmt.Sprintf("  • %s\n", path)
		}
		message += "\nThese paths may contain:\n" +
			"  - Application data and settings\n" +
			"  - Credentials and keys\n" +
			"  - Important configurations\n"
	}

	message += "\n⚠️  FILES WILL BE PERMANENTLY DELETED ⚠️\n"
	message += "This action cannot be undone.\n\n"

	if hasSensitive {
		if m.sensitiveDeleteConfirmed {
			message += "⚠️  PRESS Y AGAIN TO PERMANENTLY DELETE ⚠️"
		} else {
			message += "Press Y TWICE to confirm permanent deletion, N to cancel"
		}
	} else {
		message += "Press Y to confirm permanent deletion, N to cancel"
	}

	content := lipgloss.NewStyle().
		Width(80).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Render(message)

	return content
}

// buildDeletionTreeView creates a tree view of files to be deleted
func (m *Model) buildDeletionTreeView() string {
	if len(m.markedFiles) == 0 {
		return "  (none)\n"
	}

	// Group files by parent directory for better display
	type DirGroup struct {
		dir   string
		files []string
	}

	dirMap := make(map[string][]string)

	for path := range m.markedFiles {
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		dirMap[dir] = append(dirMap[dir], base)
	}

	var result strings.Builder
	maxLines := 12 // Show max 12 lines to keep modal from getting too tall
	lineCount := 0
	totalFiles := len(m.markedFiles)

	// Sort directories for consistent display
	dirs := make([]string, 0, len(dirMap))
	for dir := range dirMap {
		dirs = append(dirs, dir)
	}

	// Simple sort
	for i := 0; i < len(dirs); i++ {
		for j := i + 1; j < len(dirs); j++ {
			if dirs[i] > dirs[j] {
				dirs[i], dirs[j] = dirs[j], dirs[i]
			}
		}
	}

	for _, dir := range dirs {
		files := dirMap[dir]

		if lineCount >= maxLines {
			remaining := totalFiles - lineCount
			if remaining > 0 {
				result.WriteString(fmt.Sprintf("  ... and %d more file(s)\n", remaining))
			}
			break
		}

		// Show directory (abbreviated if too long)
		displayDir := dir
		if len(displayDir) > 60 {
			displayDir = "..." + displayDir[len(displayDir)-57:]
		}

		result.WriteString(fmt.Sprintf("  📁 %s\n", displayDir))
		lineCount++

		// Show files under this directory
		for i, file := range files {
			if lineCount >= maxLines {
				remaining := totalFiles - lineCount
				result.WriteString(fmt.Sprintf("     ... and %d more\n", remaining))
				break
			}

			// Truncate filename if too long
			displayFile := file
			if len(displayFile) > 55 {
				displayFile = displayFile[:52] + "..."
			}

			// Use tree characters
			if i == len(files)-1 {
				result.WriteString(fmt.Sprintf("     └─ %s\n", displayFile))
			} else {
				result.WriteString(fmt.Sprintf("     ├─ %s\n", displayFile))
			}
			lineCount++
		}
	}

	return result.String()
}

// renderDeleteProgressModal renders the deletion progress dialog
func (m *Model) renderDeleteProgressModal() string {
	progress := float64(m.deleteProgress.Current) / float64(m.deleteProgress.Total)
	progressBar := m.renderProgressBar(progress, 50)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Render("🗑️  Deleting Files...")

	content := lipgloss.NewStyle().
		Width(60).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Render(fmt.Sprintf(
			"%s\n\n"+
				"%s\n\n"+
				"Progress: %d / %d\n\n"+
				"Current file:\n%s",
			title,
			progressBar,
			m.deleteProgress.Current,
			m.deleteProgress.Total,
			m.truncatePath(m.deleteProgress.CurrentFile, 56),
		))

	return content
}

// renderDeleteSummaryModal renders the deletion summary dialog
func (m *Model) renderDeleteSummaryModal() string {
	// Show errors if any
	if len(m.deleteProgress.Errors) > 0 {
		title := lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorDanger).
			Render("⚠ Deletion Errors")

		var errorList strings.Builder
		for i, err := range m.deleteProgress.Errors {
			if i < 5 { // Show first 5 errors
				errorList.WriteString(fmt.Sprintf("  • %s\n", err.Error()))
			}
		}
		if len(m.deleteProgress.Errors) > 5 {
			errorList.WriteString(fmt.Sprintf("  ... and %d more errors\n", len(m.deleteProgress.Errors)-5))
		}

		message := fmt.Sprintf(
			"%s\n\n"+
				"Errors occurred during deletion:\n\n"+
				"%s\n"+
				"Successfully deleted: %d item(s)\n"+
				"Space reclaimed: %s\n\n"+
				"Press any key to continue",
			title,
			errorList.String(),
			m.deleteProgress.FilesDeleted,
			util.FormatBytes(m.deleteProgress.BytesDeleted),
		)

		content := lipgloss.NewStyle().
			Width(70).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorDanger).
			Render(message)

		return content
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSuccess).
		Render("✓ Deletion Complete")

	spaceReclaimed := util.FormatBytes(m.deleteProgress.BytesDeleted)

	// Build message with appropriate details
	var message string
	if m.deleteProgress.TotalFilesDeleted > m.deleteProgress.FilesDeleted {
		// Directories were deleted - show both counts
		message = fmt.Sprintf(
			"%s\n\n"+
				"Successfully deleted:\n"+
				"  • %d item(s) (files and/or directories)\n"+
				"  • %d total file(s) inside\n"+
				"  • Space reclaimed: %s\n\n"+
				"Press any key to continue",
			title,
			m.deleteProgress.FilesDeleted,
			m.deleteProgress.TotalFilesDeleted,
			spaceReclaimed,
		)
	} else {
		// Only files deleted
		message = fmt.Sprintf(
			"%s\n\n"+
				"Successfully deleted:\n"+
				"  • %d file(s)\n"+
				"  • Space reclaimed: %s\n\n"+
				"Press any key to continue",
			title,
			m.deleteProgress.FilesDeleted,
			spaceReclaimed,
		)
	}

	content := lipgloss.NewStyle().
		Width(60).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSuccess).
		Render(message)

	return content
}

// renderProgressBar renders a text progress bar
func (m *Model) renderProgressBar(progress float64, width int) string {
	filled := int(progress * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	percentage := int(progress * 100)

	return fmt.Sprintf("[%s] %d%%", bar, percentage)
}

// truncatePath truncates a path to fit within maxLen
func (m *Model) truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}
