package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"spaceforce/scanner"
	"spaceforce/util"
)

// TimelineView displays files grouped by age
type TimelineView struct {
	buckets       []*TimeBucket
	selectedIndex int
	height        int
	totalSize     int64
}

// TimeBucket represents a time period with associated files
type TimeBucket struct {
	Name      string
	StartDate time.Time
	EndDate   time.Time
	Files     []*scanner.FileNode
	TotalSize int64
	FileCount int64
}

// NewTimelineView creates a new timeline view
func NewTimelineView(root *scanner.FileNode) *TimelineView {
	tv := &TimelineView{
		height: 20,
	}
	tv.buildBuckets(root)
	return tv
}

// Init initializes the view
func (tv *TimelineView) Init() tea.Cmd {
	return nil
}

// Update handles updates
func (tv *TimelineView) Update(msg tea.Msg) (*TimelineView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if tv.selectedIndex > 0 {
				tv.selectedIndex--
			}
		case "down", "j":
			if tv.selectedIndex < len(tv.buckets)-1 {
				tv.selectedIndex++
			}
		}
	}
	return tv, nil
}

// View renders the view
func (tv *TimelineView) View() string {
	var b strings.Builder

	b.WriteString(util.TitleStyle.Render("⏰ Timeline View"))
	b.WriteString("\n")
	b.WriteString(util.SubtitleStyle.Render("Files grouped by last modified date"))
	b.WriteString("\n\n")

	// Header
	header := fmt.Sprintf("%-30s %12s %10s %8s %s",
		"Time Period", "Total Size", "Files", "Percent", "Bar")
	b.WriteString(util.HelpStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", 90))
	b.WriteString("\n")

	// Render buckets
	for i, bucket := range tv.buckets {
		line := tv.renderBucket(bucket, i == tv.selectedIndex)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(util.HelpStyle.Render("Old files may be safe to archive or delete"))

	return b.String()
}

// renderBucket renders a time bucket
func (tv *TimelineView) renderBucket(bucket *TimeBucket, selected bool) string {
	// Calculate percentage
	percentage := float64(0)
	if tv.totalSize > 0 {
		percentage = float64(bucket.TotalSize) / float64(tv.totalSize) * 100
	}

	// Create progress bar
	barWidth := 20
	filledWidth := int(percentage / 100 * float64(barWidth))
	if filledWidth > barWidth {
		filledWidth = barWidth
	}
	bar := strings.Repeat("█", filledWidth) + strings.Repeat("░", barWidth-filledWidth)

	// Build line
	line := fmt.Sprintf("%-30s %12s %10d %7.1f%% %s",
		bucket.Name,
		util.FormatBytes(bucket.TotalSize),
		bucket.FileCount,
		percentage,
		bar)

	if selected {
		return util.SelectedItemStyle.Render(line)
	}
	return util.NormalItemStyle.Render(line)
}

// buildBuckets creates time buckets and categorizes files
func (tv *TimelineView) buildBuckets(root *scanner.FileNode) {
	now := time.Now()

	// Define time buckets
	tv.buckets = []*TimeBucket{
		{
			Name:      "Last 24 hours",
			StartDate: now.Add(-24 * time.Hour),
			EndDate:   now,
			Files:     make([]*scanner.FileNode, 0),
		},
		{
			Name:      "Last week",
			StartDate: now.Add(-7 * 24 * time.Hour),
			EndDate:   now.Add(-24 * time.Hour),
			Files:     make([]*scanner.FileNode, 0),
		},
		{
			Name:      "Last month",
			StartDate: now.Add(-30 * 24 * time.Hour),
			EndDate:   now.Add(-7 * 24 * time.Hour),
			Files:     make([]*scanner.FileNode, 0),
		},
		{
			Name:      "Last 3 months",
			StartDate: now.Add(-90 * 24 * time.Hour),
			EndDate:   now.Add(-30 * 24 * time.Hour),
			Files:     make([]*scanner.FileNode, 0),
		},
		{
			Name:      "Last 6 months",
			StartDate: now.Add(-180 * 24 * time.Hour),
			EndDate:   now.Add(-90 * 24 * time.Hour),
			Files:     make([]*scanner.FileNode, 0),
		},
		{
			Name:      "Last year",
			StartDate: now.Add(-365 * 24 * time.Hour),
			EndDate:   now.Add(-180 * 24 * time.Hour),
			Files:     make([]*scanner.FileNode, 0),
		},
		{
			Name:      "Over a year ago",
			StartDate: time.Time{}, // Beginning of time
			EndDate:   now.Add(-365 * 24 * time.Hour),
			Files:     make([]*scanner.FileNode, 0),
		},
	}

	// Categorize all files
	allFiles := scanner.FlattenTree(root)
	for _, file := range allFiles {
		if file.IsDir {
			continue // Skip directories in timeline view
		}

		// Find appropriate bucket
		for _, bucket := range tv.buckets {
			if file.ModTime.After(bucket.StartDate) && file.ModTime.Before(bucket.EndDate) {
				bucket.Files = append(bucket.Files, file)
				bucket.TotalSize += file.Size
				bucket.FileCount++
				tv.totalSize += file.Size
				break
			} else if bucket.StartDate.IsZero() && file.ModTime.Before(bucket.EndDate) {
				// Handle "over a year ago" bucket
				bucket.Files = append(bucket.Files, file)
				bucket.TotalSize += file.Size
				bucket.FileCount++
				tv.totalSize += file.Size
				break
			}
		}
	}
}

// SetHeight sets the viewport height
func (tv *TimelineView) SetHeight(height int) {
	tv.height = height
}

// GetSelectedBucket returns the currently selected bucket
func (tv *TimelineView) GetSelectedBucket() *TimeBucket {
	if tv.selectedIndex < len(tv.buckets) {
		return tv.buckets[tv.selectedIndex]
	}
	return nil
}

// GetOldFiles returns files older than a certain age
func (tv *TimelineView) GetOldFiles(months int) []*scanner.FileNode {
	cutoffDate := time.Now().Add(-time.Duration(months) * 30 * 24 * time.Hour)
	oldFiles := make([]*scanner.FileNode, 0)

	for _, bucket := range tv.buckets {
		if bucket.EndDate.Before(cutoffDate) {
			oldFiles = append(oldFiles, bucket.Files...)
		}
	}

	return oldFiles
}
