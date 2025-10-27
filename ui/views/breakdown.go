package views

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"spaceforce/scanner"
	"spaceforce/util"
)

// BreakdownView displays file type breakdown statistics
type BreakdownView struct {
	stats         *scanner.DirStats
	types         []*scanner.TypeStats
	selectedIndex int
	height        int
	totalSize     int64
}

// NewBreakdownView creates a new breakdown view
func NewBreakdownView(root *scanner.FileNode) *BreakdownView {
	stats := scanner.CalculateStats(root)
	bv := &BreakdownView{
		stats:     stats,
		types:     make([]*scanner.TypeStats, 0),
		height:    20,
		totalSize: stats.TotalSize,
	}

	// Convert map to sorted slice
	for _, typeStats := range stats.TypeBreakdown {
		bv.types = append(bv.types, typeStats)
	}

	// Sort by total size descending
	sort.Slice(bv.types, func(i, j int) bool {
		return bv.types[i].TotalSize > bv.types[j].TotalSize
	})

	return bv
}

// Init initializes the view
func (bv *BreakdownView) Init() tea.Cmd {
	return nil
}

// Update handles updates
func (bv *BreakdownView) Update(msg tea.Msg) (*BreakdownView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if bv.selectedIndex > 0 {
				bv.selectedIndex--
			}
		case "down", "j":
			if bv.selectedIndex < len(bv.types)-1 {
				bv.selectedIndex++
			}
		}
	}
	return bv, nil
}

// View renders the view
func (bv *BreakdownView) View() string {
	var b strings.Builder

	b.WriteString(util.TitleStyle.Render("📈 File Type Breakdown"))
	b.WriteString("\n")
	b.WriteString(util.SubtitleStyle.Render(fmt.Sprintf("Total: %s across %d files in %d directories",
		util.FormatBytes(bv.stats.TotalSize), bv.stats.FileCount, bv.stats.DirCount)))
	b.WriteString("\n\n")

	// Header
	header := fmt.Sprintf("%-20s %12s %10s %8s %s",
		"Type", "Total Size", "Files", "Percent", "Bar")
	b.WriteString(util.HelpStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 90))
	b.WriteString("\n")

	// Reserve lines for title (2), subtitle (3), header (2), separator (2), summary (2)
	// Total chrome: 9 lines + 2 for optional summary = 11 lines worst case
	contentHeight := bv.height - 11
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Calculate viewport
	start := bv.selectedIndex - contentHeight/2
	if start < 0 {
		start = 0
	}
	end := start + contentHeight
	if end > len(bv.types) {
		end = len(bv.types)
		start = end - contentHeight
		if start < 0 {
			start = 0
		}
	}

	// Render items
	for i := start; i < end && i < len(bv.types); i++ {
		typeStats := bv.types[i]
		line := bv.renderTypeStats(typeStats, i == bv.selectedIndex)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Summary
	if len(bv.types) > contentHeight {
		b.WriteString("\n")
		b.WriteString(util.HelpStyle.Render(fmt.Sprintf("Showing %d-%d of %d types",
			start+1, end, len(bv.types))))
	}

	return b.String()
}

// renderTypeStats renders statistics for a file type
func (bv *BreakdownView) renderTypeStats(typeStats *scanner.TypeStats, selected bool) string {
	// Calculate percentage
	percentage := float64(0)
	if bv.totalSize > 0 {
		percentage = float64(typeStats.TotalSize) / float64(bv.totalSize) * 100
	}

	// Create progress bar
	barWidth := 20
	filledWidth := int(percentage / 100 * float64(barWidth))
	if filledWidth > barWidth {
		filledWidth = barWidth
	}
	bar := strings.Repeat("█", filledWidth) + strings.Repeat("░", barWidth-filledWidth)

	// Format type name
	typeName := typeStats.Extension
	if typeName == "directory" {
		typeName = "[directories]"
	} else if typeName == "no-extension" {
		typeName = "[no extension]"
	}
	if len(typeName) > 18 {
		typeName = typeName[:15] + "..."
	}

	// Build line
	line := fmt.Sprintf("%-20s %12s %10d %7.1f%% %s",
		typeName,
		util.FormatBytes(typeStats.TotalSize),
		typeStats.FileCount,
		percentage,
		bar)

	if selected {
		return util.SelectedItemStyle.Render(line)
	}
	return util.NormalItemStyle.Render(line)
}

// SetHeight sets the viewport height
func (bv *BreakdownView) SetHeight(height int) {
	bv.height = height
}

// GetSelectedType returns the currently selected type stats
func (bv *BreakdownView) GetSelectedType() *scanner.TypeStats {
	if bv.selectedIndex < len(bv.types) {
		return bv.types[bv.selectedIndex]
	}
	return nil
}

// GetCategoryDescription returns a description for common file categories
func GetCategoryDescription(extension string) string {
	categories := map[string]string{
		".jpg":       "Images",
		".jpeg":      "Images",
		".png":       "Images",
		".gif":       "Images",
		".mp4":       "Videos",
		".mov":       "Videos",
		".avi":       "Videos",
		".mkv":       "Videos",
		".mp3":       "Audio",
		".wav":       "Audio",
		".flac":      "Audio",
		".pdf":       "Documents",
		".doc":       "Documents",
		".docx":      "Documents",
		".txt":       "Text Files",
		".log":       "Log Files",
		".zip":       "Archives",
		".tar":       "Archives",
		".gz":        "Archives",
		".dmg":       "Disk Images",
		".iso":       "Disk Images",
		".app":       "Applications",
		".pkg":       "Installers",
		".cache":     "Cache Files",
		"directory":  "Directories",
		"no-extension": "Files without extension",
	}

	if desc, ok := categories[extension]; ok {
		return desc
	}
	return "Other Files"
}
