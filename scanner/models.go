package scanner

import (
	"path/filepath"
	"time"
)

// FileNode represents a file or directory in the filesystem tree
type FileNode struct {
	Path         string
	Name         string
	Size         int64
	IsDir        bool
	ModTime      time.Time
	Children     []*FileNode
	Parent       *FileNode
	FileType     string // Extension or "directory"
	IsProtected  bool   // Whether this file is protected from deletion
}

// DirStats holds aggregate statistics for a directory
type DirStats struct {
	TotalSize      int64
	FileCount      int64
	DirCount       int64
	LargestFiles   []*FileNode
	TypeBreakdown  map[string]*TypeStats
}

// TypeStats holds statistics for a particular file type
type TypeStats struct {
	Extension  string
	TotalSize  int64
	FileCount  int64
	Files      []*FileNode
}

// ScanProgress represents the current state of a scan
type ScanProgress struct {
	CurrentPath        string
	FilesScanned       int64
	BytesScanned       int64
	TotalBytes         int64  // Estimated total bytes to scan
	Errors             []error
	Complete           bool
	ICloudFilesSkipped int64 // Count of .icloud placeholder files skipped
}

// NewFileNode creates a new file node
func NewFileNode(path string, size int64, isDir bool, modTime time.Time) *FileNode {
	name := filepath.Base(path)
	ext := filepath.Ext(path)
	if ext == "" {
		if isDir {
			ext = "directory"
		} else {
			ext = "no-extension"
		}
	}

	return &FileNode{
		Path:     path,
		Name:     name,
		Size:     size,
		IsDir:    isDir,
		ModTime:  modTime,
		Children: make([]*FileNode, 0),
		FileType: ext,
	}
}

// AddChild adds a child node and updates the parent reference
func (n *FileNode) AddChild(child *FileNode) {
	child.Parent = n
	n.Children = append(n.Children, child)
}

// TotalSize recursively calculates the total size including all children
func (n *FileNode) TotalSize() int64 {
	if !n.IsDir {
		return n.Size
	}

	total := int64(0)
	for _, child := range n.Children {
		total += child.TotalSize()
	}
	return total
}

// FileCount recursively counts all files in this tree
func (n *FileNode) FileCount() int64 {
	if !n.IsDir {
		return 1
	}

	count := int64(0)
	for _, child := range n.Children {
		count += child.FileCount()
	}
	return count
}
