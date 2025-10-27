package views

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"spaceforce/scanner"
	"spaceforce/util"
)

// TreeSortBy defines how tree items are sorted
type TreeSortBy int

const (
	TreeSortByName TreeSortBy = iota
	TreeSortBySize
)

// TreeView displays files in a hierarchical tree structure
type TreeView struct {
	root          *scanner.FileNode
	displayRoot   *scanner.FileNode                // Current root being displayed (for zoom)
	expandedDirs  map[string]bool
	selectedIndex int
	visibleItems  []*treeItem
	height        int
	width         int                               // Terminal width for dynamic rendering
	sortBy        TreeSortBy
	markedFiles   map[string]*scanner.FileNode     // Files marked for deletion
	sortedCache   map[string][]*scanner.FileNode   // Cache of sorted children by path
	lastSortMode  TreeSortBy                       // Track when sort mode changes
}

type treeItem struct {
	node   *scanner.FileNode
	depth  int
	index  int
	hasChildren bool
	isExpanded bool
}

// NewTreeView creates a new tree view
func NewTreeView(root *scanner.FileNode) *TreeView {
	tv := &TreeView{
		root:         root,
		displayRoot:  root,
		expandedDirs: make(map[string]bool),
		sortedCache:  make(map[string][]*scanner.FileNode),
		height:       20,
		width:        80, // Default width, will be updated by SetWidth
		sortBy:       TreeSortByName,
		lastSortMode: TreeSortByName,
	}
	tv.expandedDirs[root.Path] = true // Expand root by default
	tv.rebuildVisibleItems()
	return tv
}

// Init initializes the tree view
func (tv *TreeView) Init() tea.Cmd {
	return nil
}

// Update handles tree view updates
func (tv *TreeView) Update(msg tea.Msg) (*TreeView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if tv.selectedIndex > 0 {
				tv.selectedIndex--
			}
		case "down", "j":
			if tv.selectedIndex < len(tv.visibleItems)-1 {
				tv.selectedIndex++
			}
		case "enter", " ":
			// Toggle expansion
			if tv.selectedIndex < len(tv.visibleItems) {
				item := tv.visibleItems[tv.selectedIndex]
				if item.node.IsDir {
					tv.expandedDirs[item.node.Path] = !tv.expandedDirs[item.node.Path]
					tv.rebuildVisibleItems()
				}
			}
		case "right", "l":
			// Expand directory
			if tv.selectedIndex < len(tv.visibleItems) {
				item := tv.visibleItems[tv.selectedIndex]
				if item.node.IsDir {
					tv.expandedDirs[item.node.Path] = true
					tv.rebuildVisibleItems()
				}
			}
		case "left", "h":
			// Collapse directory
			if tv.selectedIndex < len(tv.visibleItems) {
				item := tv.visibleItems[tv.selectedIndex]
				if item.node.IsDir {
					tv.expandedDirs[item.node.Path] = false
					tv.rebuildVisibleItems()
				}
			}
		case "s":
			// Toggle sort
			if tv.sortBy == TreeSortByName {
				tv.sortBy = TreeSortBySize
			} else {
				tv.sortBy = TreeSortByName
			}
			// Clear cache when sort mode changes
			tv.sortedCache = make(map[string][]*scanner.FileNode)
			tv.lastSortMode = tv.sortBy
			tv.rebuildVisibleItems()
		case "z":
			// Zoom into selected directory
			if tv.selectedIndex < len(tv.visibleItems) {
				item := tv.visibleItems[tv.selectedIndex]
				if item.node.IsDir {
					tv.displayRoot = item.node
					tv.expandedDirs[item.node.Path] = true
					tv.selectedIndex = 0
					tv.rebuildVisibleItems()
				}
			}
		case "u":
			// Zoom out to parent directory
			if tv.displayRoot != tv.root {
				// Find parent of current displayRoot
				parent := tv.findParent(tv.root, tv.displayRoot)
				if parent != nil {
					tv.displayRoot = parent
				} else {
					tv.displayRoot = tv.root
				}
				tv.selectedIndex = 0
				tv.rebuildVisibleItems()
			}
		}
	}
	return tv, nil
}

// View renders the tree view
func (tv *TreeView) View() string {
	if tv.root == nil {
		return "No data to display"
	}

	var b strings.Builder

	// Build title with sort indicator and zoom indicator
	title := "📁 Directory Tree"
	sortIndicator := ""
	switch tv.sortBy {
	case TreeSortBySize:
		sortIndicator = " (sorted by size)"
	case TreeSortByName:
		sortIndicator = " (sorted by name)"
	}

	// Add zoom indicator if we're zoomed into a subdirectory
	zoomIndicator := ""
	if tv.displayRoot != tv.root {
		dirName := tv.displayRoot.Name
		// Truncate long directory names to prevent wrapping
		if len(dirName) > 30 {
			dirName = dirName[:27] + "..."
		}
		zoomIndicator = " [zoomed: " + dirName + "]"
	}

	// Truncate entire title if needed (max ~70 chars to be safe)
	fullTitle := title + sortIndicator + zoomIndicator
	if len(fullTitle) > 70 {
		fullTitle = fullTitle[:67] + "..."
	}

	b.WriteString(util.TitleStyle.Render(fullTitle))
	b.WriteString("\n\n")

	// Calculate content height - now simple since we removed file counts to prevent wrapping
	// Tree view outputs: title(3) + items(contentHeight) + scroll(2) = contentHeight + 5
	// So: contentHeight + 5 <= tv.height → contentHeight = tv.height - 5
	// Use tv.height - 6 to be slightly conservative
	contentHeight := tv.height - 6
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Calculate viewport
	start := tv.selectedIndex - contentHeight/2
	if start < 0 {
		start = 0
	}
	end := start + contentHeight
	if end > len(tv.visibleItems) {
		end = len(tv.visibleItems)
		start = end - contentHeight
		if start < 0 {
			start = 0
		}
	}

	// Render visible items
	for i := start; i < end && i < len(tv.visibleItems); i++ {
		item := tv.visibleItems[i]
		line := tv.renderItem(item, i == tv.selectedIndex)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Show scroll indicator
	if len(tv.visibleItems) > contentHeight {
		b.WriteString("\n")
		b.WriteString(util.HelpStyle.Render(fmt.Sprintf("Showing %d-%d of %d items",
			start+1, end, len(tv.visibleItems))))
	}

	return b.String()
}

// renderItem renders a single tree item
func (tv *TreeView) renderItem(item *treeItem, selected bool) string {
	var b strings.Builder

	// Indentation
	indent := strings.Repeat("  ", item.depth)
	b.WriteString(indent)

	// Expansion indicator
	if item.node.IsDir {
		if item.isExpanded {
			b.WriteString("▼ ")
		} else {
			b.WriteString("▶ ")
		}
	} else {
		b.WriteString("  ")
	}

	// Icon and mark indicator
	if item.node.IsDir {
		b.WriteString("📁 ")
	} else {
		b.WriteString("📄 ")
	}

	// Mark indicator
	isMarked := false
	if tv.markedFiles != nil {
		_, isMarked = tv.markedFiles[item.node.Path]
	}
	markWidth := 0
	if isMarked {
		b.WriteString("[✓] ")
		markWidth = 4
	}

	// Calculate available width for name + file count
	// Total width - (indent + expansion + icon + mark + size + padding)
	indentWidth := len(indent)
	fixedWidth := indentWidth + 2 + 2 + markWidth + 1 + 10 + 2 // expansion(2) + icon(2) + space(1) + size(~10) + padding(2)
	availableWidth := tv.width - fixedWidth
	if availableWidth < 20 {
		availableWidth = 20 // Minimum
	}
	if availableWidth > 140 {
		availableWidth = 140 // Maximum (still need to keep lines reasonable)
	}

	// Name and file count
	nameStyle := util.NormalItemStyle
	if selected {
		nameStyle = util.SelectedItemStyle
	}

	name := item.node.Name

	// Build the complete name string with file count if applicable
	var nameWithCount string
	if item.node.IsDir && tv.width > 100 {
		fileCount := item.node.FileCount()
		if fileCount > 0 {
			// Add file count right after name
			nameWithCount = fmt.Sprintf("%s (%d files)", name, fileCount)
		} else {
			nameWithCount = name
		}
	} else {
		nameWithCount = name
	}

	// Truncate if too long
	if len(nameWithCount) > availableWidth {
		nameWithCount = nameWithCount[:availableWidth-3] + "..."
	}

	// Render with padding to align size column
	b.WriteString(nameStyle.Width(availableWidth).Render(nameWithCount))

	// Size (right-aligned in its column)
	size := item.node.TotalSize()
	b.WriteString(" ")
	sizeStr := util.FormatBytes(size)
	b.WriteString(sizeStr)

	return b.String()
}

// rebuildVisibleItems rebuilds the list of visible items based on expansion state
func (tv *TreeView) rebuildVisibleItems() {
	tv.visibleItems = make([]*treeItem, 0)
	tv.buildVisibleItemsRecursive(tv.displayRoot, 0, 0)
}

// findParent finds the parent node of target within the tree rooted at node
func (tv *TreeView) findParent(node *scanner.FileNode, target *scanner.FileNode) *scanner.FileNode {
	if node == nil || target == nil {
		return nil
	}

	// Check if any of node's children is the target
	for _, child := range node.Children {
		if child == target {
			return node
		}
	}

	// Recursively search in children
	for _, child := range node.Children {
		if child.IsDir {
			parent := tv.findParent(child, target)
			if parent != nil {
				return parent
			}
		}
	}

	return nil
}

func (tv *TreeView) buildVisibleItemsRecursive(node *scanner.FileNode, depth int, index int) int {
	isExpanded := tv.expandedDirs[node.Path]

	item := &treeItem{
		node:        node,
		depth:       depth,
		index:       len(tv.visibleItems),
		hasChildren: len(node.Children) > 0,
		isExpanded:  isExpanded,
	}
	tv.visibleItems = append(tv.visibleItems, item)

	if node.IsDir && isExpanded && len(node.Children) > 0 {
		// Check cache first
		children, cached := tv.sortedCache[node.Path]
		if !cached {
			// Not in cache - sort and cache it
			children = make([]*scanner.FileNode, len(node.Children))
			copy(children, node.Children)
			tv.sortChildren(children)
			tv.sortedCache[node.Path] = children
		}

		for _, child := range children {
			index = tv.buildVisibleItemsRecursive(child, depth+1, index+1)
		}
	}

	return index
}

// sortChildren sorts a slice of FileNodes based on current sort settings
func (tv *TreeView) sortChildren(children []*scanner.FileNode) {
	switch tv.sortBy {
	case TreeSortBySize:
		sort.Slice(children, func(i, j int) bool {
			// Directories first, then sort by total size descending
			if children[i].IsDir != children[j].IsDir {
				return children[i].IsDir
			}
			return children[i].TotalSize() > children[j].TotalSize()
		})
	case TreeSortByName:
		sort.Slice(children, func(i, j int) bool {
			// Directories first, then sort by name
			if children[i].IsDir != children[j].IsDir {
				return children[i].IsDir
			}
			return children[i].Name < children[j].Name
		})
	}
}

// SetHeight sets the viewport height
func (tv *TreeView) SetHeight(height int) {
	tv.height = height
}

// SetWidth sets the viewport width
func (tv *TreeView) SetWidth(width int) {
	tv.width = width
}

// SetMarkedFiles updates the marked files map
func (tv *TreeView) SetMarkedFiles(markedFiles map[string]*scanner.FileNode) {
	tv.markedFiles = markedFiles
}

// GetSelectedNode returns the currently selected node
func (tv *TreeView) GetSelectedNode() *scanner.FileNode {
	if tv.selectedIndex < len(tv.visibleItems) {
		return tv.visibleItems[tv.selectedIndex].node
	}
	return nil
}
