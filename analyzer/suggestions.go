package analyzer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"spaceforce/safety"
	"spaceforce/scanner"
)

// Suggestion represents a cleanup suggestion
type Suggestion struct {
	Path        string
	Description string
	Reason      string
	Savings     int64
	RiskLevel   int // 0=safe, 1=low, 2=medium, 3=high
	Category    string
	Files       []*scanner.FileNode
}

// SuggestionEngine generates cleanup suggestions
type SuggestionEngine struct {
	protector *safety.Protector
	root      *scanner.FileNode
}

// NewSuggestionEngine creates a new suggestion engine
func NewSuggestionEngine(root *scanner.FileNode) *SuggestionEngine {
	return &SuggestionEngine{
		protector: safety.NewProtector(),
		root:      root,
	}
}

// GenerateSuggestions analyzes the filesystem and generates cleanup suggestions
func (se *SuggestionEngine) GenerateSuggestions() []*Suggestion {
	suggestions := make([]*Suggestion, 0)

	// Check common bloat locations
	suggestions = append(suggestions, se.checkBloatLocations()...)

	// Find old files
	suggestions = append(suggestions, se.findOldFiles()...)

	// Find large cache directories
	suggestions = append(suggestions, se.findLargeCaches()...)

	// Find old log files
	suggestions = append(suggestions, se.findOldLogs()...)

	// Find duplicate large files (simplified - just by size)
	suggestions = append(suggestions, se.findDuplicateSizes()...)

	// Development-specific suggestions
	suggestions = append(suggestions, se.findDevelopmentBloat()...)

	// Sort by potential savings
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Savings > suggestions[j].Savings
	})

	return suggestions
}

// checkBloatLocations checks known bloat locations
func (se *SuggestionEngine) checkBloatLocations() []*Suggestion {
	suggestions := make([]*Suggestion, 0)
	bloatLocations := safety.GetCommonBloatLocations()

	homeDir, _ := os.UserHomeDir()

	for _, location := range bloatLocations {
		// Expand ~ to home directory
		path := location.Path
		if strings.HasPrefix(path, "~") {
			path = strings.Replace(path, "~", homeDir, 1)
		}

		// Check if path exists and calculate size
		totalSize := int64(0)
		files := make([]*scanner.FileNode, 0)

		// Find matching nodes in our tree
		matchingNodes := se.findNodesByPath(se.root, path)
		for _, node := range matchingNodes {
			size := node.TotalSize()
			if size > 100*1024*1024 { // Only suggest if > 100MB
				totalSize += size
				files = append(files, node)
			}
		}

		if totalSize > 0 {
			suggestions = append(suggestions, &Suggestion{
				Path:        path,
				Description: location.Description,
				Reason:      location.Reason,
				Savings:     totalSize,
				RiskLevel:   location.RiskLevel,
				Category:    "Known Bloat",
				Files:       files,
			})
		}
	}

	return suggestions
}

// findOldFiles finds files that haven't been modified in a long time
func (se *SuggestionEngine) findOldFiles() []*Suggestion {
	cutoffDate := time.Now().Add(-365 * 24 * time.Hour) // 1 year ago
	oldFiles := make([]*scanner.FileNode, 0)
	totalSize := int64(0)

	allFiles := scanner.FlattenTree(se.root)
	for _, file := range allFiles {
		if !file.IsDir && file.ModTime.Before(cutoffDate) && file.Size > 10*1024*1024 {
			// Check if safe to delete
			if safe, _ := se.protector.IsSafeToDelete(file.Path); safe {
				oldFiles = append(oldFiles, file)
				totalSize += file.Size
			}
		}
	}

	if len(oldFiles) > 0 {
		return []*Suggestion{
			{
				Path:        "Multiple locations",
				Description: "Files not modified in over 1 year",
				Reason:      "Old files may no longer be needed",
				Savings:     totalSize,
				RiskLevel:   1,
				Category:    "Old Files",
				Files:       oldFiles,
			},
		}
	}

	return nil
}

// findLargeCaches finds large cache directories
func (se *SuggestionEngine) findLargeCaches() []*Suggestion {
	suggestions := make([]*Suggestion, 0)
	allFiles := scanner.FlattenTree(se.root)

	cacheNodes := make([]*scanner.FileNode, 0)
	for _, file := range allFiles {
		if file.IsDir && se.protector.IsCache(file.Path) {
			size := file.TotalSize()
			if size > 100*1024*1024 { // > 100MB
				cacheNodes = append(cacheNodes, file)
			}
		}
	}

	if len(cacheNodes) > 0 {
		totalSize := int64(0)
		for _, node := range cacheNodes {
			totalSize += node.TotalSize()
		}

		suggestions = append(suggestions, &Suggestion{
			Path:        "Multiple cache directories",
			Description: "Large cache directories",
			Reason:      "Applications will recreate caches as needed",
			Savings:     totalSize,
			RiskLevel:   0,
			Category:    "Caches",
			Files:       cacheNodes,
		})
	}

	return suggestions
}

// findOldLogs finds old log files
func (se *SuggestionEngine) findOldLogs() []*Suggestion {
	cutoffDate := time.Now().Add(-90 * 24 * time.Hour) // 3 months ago
	logFiles := make([]*scanner.FileNode, 0)
	totalSize := int64(0)

	allFiles := scanner.FlattenTree(se.root)
	for _, file := range allFiles {
		if !file.IsDir && se.protector.IsLogFile(file.Path) && file.ModTime.Before(cutoffDate) {
			logFiles = append(logFiles, file)
			totalSize += file.Size
		}
	}

	if len(logFiles) > 0 && totalSize > 100*1024*1024 {
		return []*Suggestion{
			{
				Path:        "Multiple locations",
				Description: "Old log files (>3 months)",
				Reason:      "Old logs are rarely needed",
				Savings:     totalSize,
				RiskLevel:   0,
				Category:    "Logs",
				Files:       logFiles,
			},
		}
	}

	return nil
}

// findDuplicateSizes finds files with the same size (potential duplicates)
func (se *SuggestionEngine) findDuplicateSizes() []*Suggestion {
	allFiles := scanner.FlattenTree(se.root)
	sizeMap := make(map[int64][]*scanner.FileNode)

	// Group files by size
	for _, file := range allFiles {
		if !file.IsDir && file.Size > 10*1024*1024 { // Only large files
			sizeMap[file.Size] = append(sizeMap[file.Size], file)
		}
	}

	suggestions := make([]*Suggestion, 0)
	for size, files := range sizeMap {
		if len(files) > 1 {
			// Potential duplicates
			totalWaste := size * int64(len(files)-1)
			if totalWaste > 100*1024*1024 {
				suggestions = append(suggestions, &Suggestion{
					Path:        "Multiple locations",
					Description: "Files with identical sizes (potential duplicates)",
					Reason:      "These files might be duplicates - review before deleting",
					Savings:     totalWaste,
					RiskLevel:   2,
					Category:    "Potential Duplicates",
					Files:       files,
				})
			}
		}
	}

	return suggestions
}

// findDevelopmentBloat finds development-related bloat
func (se *SuggestionEngine) findDevelopmentBloat() []*Suggestion {
	suggestions := make([]*Suggestion, 0)
	allFiles := scanner.FlattenTree(se.root)

	devPaths := map[string]string{
		"node_modules":  "NPM package dependencies",
		"target":        "Rust build artifacts",
		"build":         "Build artifacts",
		"dist":          "Distribution build artifacts",
		".gradle":       "Gradle cache",
		".m2":           "Maven cache",
		"DerivedData":   "Xcode build data",
		"__pycache__":   "Python bytecode cache",
		".pytest_cache": "Pytest cache",
	}

	for _, file := range allFiles {
		if !file.IsDir {
			continue
		}

		basename := filepath.Base(file.Path)
		if description, found := devPaths[basename]; found {
			size := file.TotalSize()
			if size > 100*1024*1024 {
				suggestions = append(suggestions, &Suggestion{
					Path:        file.Path,
					Description: description,
					Reason:      "Build tools will regenerate these as needed",
					Savings:     size,
					RiskLevel:   0,
					Category:    "Development",
					Files:       []*scanner.FileNode{file},
				})
			}
		}
	}

	return suggestions
}

// findNodesByPath finds nodes matching a path pattern
func (se *SuggestionEngine) findNodesByPath(node *scanner.FileNode, pathPattern string) []*scanner.FileNode {
	matches := make([]*scanner.FileNode, 0)

	if strings.Contains(node.Path, pathPattern) || matchesWildcard(node.Path, pathPattern) {
		matches = append(matches, node)
	}

	for _, child := range node.Children {
		matches = append(matches, se.findNodesByPath(child, pathPattern)...)
	}

	return matches
}

// matchesWildcard checks if a path matches a wildcard pattern (simplified)
func matchesWildcard(path, pattern string) bool {
	// Very simplified wildcard matching - just handle * in the pattern
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(path, parts[0]) && strings.HasSuffix(path, parts[1])
		}
	}
	return false
}
