package views

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"spaceforce/safety"
	"spaceforce/scanner"
	"spaceforce/util"
)

// TopListView displays the largest files/folders sorted by size
type TopListView struct {
	allItems      []*scanner.FileNode              // Full unfiltered list
	items         []*scanner.FileNode              // Filtered/sorted display list
	selectedIndex int
	height        int
	sortMode      string                           // "size", "name", "modified"
	protector     *safety.Protector
	showFiles     bool
	showDirs      bool
	markedFiles   map[string]*scanner.FileNode // Files marked for deletion
}

// NewTopListView creates a new top list view
func NewTopListView(root *scanner.FileNode) *TopListView {
	tlv := &TopListView{
		height:    20,
		sortMode:  "size",
		protector: safety.NewProtector(),
		showFiles: true,
		showDirs:  true,
	}
	tlv.buildItemList(root)
	return tlv
}

// Init initializes the view
func (tlv *TopListView) Init() tea.Cmd {
	return nil
}

// Update handles updates
func (tlv *TopListView) Update(msg tea.Msg) (*TopListView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if tlv.selectedIndex > 0 {
				tlv.selectedIndex--
			}
		case "down", "j":
			if tlv.selectedIndex < len(tlv.items)-1 {
				tlv.selectedIndex++
			}
		case "s":
			// Toggle sort mode
			switch tlv.sortMode {
			case "size":
				tlv.sortMode = "name"
			case "name":
				tlv.sortMode = "modified"
			case "modified":
				tlv.sortMode = "size"
			}
			tlv.sortItems()
		case "f":
			// Toggle files
			tlv.showFiles = !tlv.showFiles
			tlv.filterItems()
		case "d":
			// Toggle directories
			tlv.showDirs = !tlv.showDirs
			tlv.filterItems()
		}
	}
	return tlv, nil
}

// View renders the view
func (tlv *TopListView) View() string {
	var b strings.Builder

	b.WriteString(util.TitleStyle.Render("📊 Largest Items"))
	b.WriteString("\n")
	b.WriteString(util.SubtitleStyle.Render(fmt.Sprintf("Sort: %s | Files: %t | Dirs: %t",
		tlv.sortMode, tlv.showFiles, tlv.showDirs)))
	b.WriteString("\n\n")

	// Header
	header := fmt.Sprintf("%-50s %12s %10s %15s",
		"Path", "Size", "Type", "Safety")
	b.WriteString(util.HelpStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 90))
	b.WriteString("\n")

	// Reserve lines for title (2), subtitle (3), header (2), separator (2), footer (2)
	// Total chrome: 9 lines + 2 for optional footer = 11 lines worst case
	contentHeight := tlv.height - 11
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Calculate viewport
	start := tlv.selectedIndex - contentHeight/2
	if start < 0 {
		start = 0
	}
	end := start + contentHeight
	if end > len(tlv.items) {
		end = len(tlv.items)
		start = end - contentHeight
		if start < 0 {
			start = 0
		}
	}

	// Render items
	for i := start; i < end && i < len(tlv.items); i++ {
		item := tlv.items[i]
		line := tlv.renderItem(item, i == tlv.selectedIndex)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Footer
	if len(tlv.items) > contentHeight {
		b.WriteString("\n")
		b.WriteString(util.HelpStyle.Render(fmt.Sprintf("Showing %d-%d of %d items",
			start+1, end, len(tlv.items))))
	}

	return b.String()
}

// renderItem renders a single item
func (tlv *TopListView) renderItem(node *scanner.FileNode, selected bool) string {
	// Mark indicator
	markIndicator := "   "
	if tlv.markedFiles != nil {
		if _, isMarked := tlv.markedFiles[node.Path]; isMarked {
			markIndicator = "[✓]"
		}
	}

	// Get relative or shortened path
	path := node.Path
	if len(path) > 42 {
		path = "..." + path[len(path)-39:]
	}

	// Type
	itemType := "File"
	if node.IsDir {
		itemType = "Dir"
	}

	// Safety check
	riskLevel := tlv.protector.GetRiskLevel(node.Path)
	safetyStr := util.FormatSafetyLevel(riskLevel)

	// Build line
	line := fmt.Sprintf("%s %-47s %12s %10s %15s",
		markIndicator,
		path,
		util.FormatBytes(node.TotalSize()),
		itemType,
		safetyStr)

	if selected {
		return util.SelectedItemStyle.Render(line)
	}
	return util.NormalItemStyle.Render(line)
}

// buildItemList builds the flat list from the tree
func (tlv *TopListView) buildItemList(root *scanner.FileNode) {
	tlv.allItems = scanner.FlattenTree(root)
	tlv.filterItems()
	tlv.sortItems()
}

// filterItems filters the list based on show flags
func (tlv *TopListView) filterItems() {
	if tlv.showFiles && tlv.showDirs {
		// No filtering needed - use all items
		tlv.items = tlv.allItems
		return
	}

	// Filter from the full unfiltered list
	filtered := make([]*scanner.FileNode, 0)
	for _, item := range tlv.allItems {
		if item.IsDir && tlv.showDirs {
			filtered = append(filtered, item)
		} else if !item.IsDir && tlv.showFiles {
			filtered = append(filtered, item)
		}
	}
	tlv.items = filtered

	// Adjust selection if needed
	if tlv.selectedIndex >= len(tlv.items) {
		tlv.selectedIndex = len(tlv.items) - 1
	}
	if tlv.selectedIndex < 0 {
		tlv.selectedIndex = 0
	}
}

// sortItems sorts the items based on the current sort mode
func (tlv *TopListView) sortItems() {
	switch tlv.sortMode {
	case "size":
		sort.Slice(tlv.items, func(i, j int) bool {
			return tlv.items[i].TotalSize() > tlv.items[j].TotalSize()
		})
	case "name":
		sort.Slice(tlv.items, func(i, j int) bool {
			return filepath.Base(tlv.items[i].Path) < filepath.Base(tlv.items[j].Path)
		})
	case "modified":
		sort.Slice(tlv.items, func(i, j int) bool {
			return tlv.items[i].ModTime.After(tlv.items[j].ModTime)
		})
	}
}

// SetHeight sets the viewport height
func (tlv *TopListView) SetHeight(height int) {
	tlv.height = height
}

// SetMarkedFiles updates the marked files map
func (tlv *TopListView) SetMarkedFiles(markedFiles map[string]*scanner.FileNode) {
	tlv.markedFiles = markedFiles
}

// GetSelectedNode returns the currently selected node
func (tlv *TopListView) GetSelectedNode() *scanner.FileNode {
	if tlv.selectedIndex < len(tlv.items) {
		return tlv.items[tlv.selectedIndex]
	}
	return nil
}
