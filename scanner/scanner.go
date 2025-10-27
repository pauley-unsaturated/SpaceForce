package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"spaceforce/safety"
)

const (
	// dirReadTimeout is the maximum time to wait for a directory read
	// If a directory takes longer than this, it's likely on a slow/stuck network volume
	dirReadTimeout = 5 * time.Second
)

// Scanner handles filesystem scanning operations
type Scanner struct {
	root              *FileNode
	progress          *ScanProgress
	mu                sync.Mutex
	lastProgressUpdate int64
	volumeChecker     *safety.VolumeChecker
	skippedVolumes    []string
	volumesMu         sync.Mutex
	workerSem         chan struct{} // Semaphore to limit concurrent workers
	startDeviceID     uint64        // Device ID of the starting directory
	oneFilesystem     bool          // Stay on one filesystem (like du -x)
	seenInodes        map[uint64]map[uint64]bool // device_id -> inode -> seen (for deduplication)
	seenInodesMu      sync.Mutex
}

// NewScanner creates a new scanner instance
func NewScanner() *Scanner {
	// Limit to 8 concurrent workers for optimal performance
	// Rationale: Most Macs have 4-12 cores. 8 workers provides:
	// - Good parallelism on multi-core systems
	// - Avoids excessive goroutine overhead
	// - File I/O is often the bottleneck, not CPU
	// - Prevents system resource exhaustion
	maxWorkers := 8

	return &Scanner{
		progress: &ScanProgress{
			Errors: make([]error, 0),
		},
		volumeChecker:  safety.NewVolumeChecker(true), // Skip network by default
		skippedVolumes: make([]string, 0),
		workerSem:      make(chan struct{}, maxWorkers),
		oneFilesystem:  true, // Stay on one filesystem by default (like du -x)
		seenInodes:     make(map[uint64]map[uint64]bool),
	}
}

// SetSkipNetwork sets whether to skip network volumes
func (s *Scanner) SetSkipNetwork(skip bool) {
	s.volumeChecker = safety.NewVolumeChecker(skip)
}

// SetOneFilesystem sets whether to stay on one filesystem (like du -x)
func (s *Scanner) SetOneFilesystem(oneFS bool) {
	s.oneFilesystem = oneFS
}

// GetSkippedVolumes returns the list of skipped network volumes
func (s *Scanner) GetSkippedVolumes() []string {
	s.volumesMu.Lock()
	defer s.volumesMu.Unlock()
	return s.skippedVolumes
}

// Scan walks the filesystem starting from rootPath and builds a tree
func (s *Scanner) Scan(ctx context.Context, rootPath string, progressChan chan<- ScanProgress) (*FileNode, error) {
	// Normalize the path
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access path: %w", err)
	}

	// Get the starting device ID for filesystem boundary detection
	if s.oneFilesystem {
		devID, err := getDeviceID(absPath)
		if err != nil {
			return nil, fmt.Errorf("cannot get device ID: %w", err)
		}
		s.startDeviceID = devID
	}

	// Create root node
	s.root = NewFileNode(absPath, info.Size(), info.IsDir(), info.ModTime())

	// Start scanning (parallel for better performance)
	if info.IsDir() {
		s.scanDirectoryParallel(ctx, s.root, progressChan, 0)
	}

	// Check if cancelled
	if ctx.Err() != nil {
		s.mu.Lock()
		s.progress.Complete = false
		s.mu.Unlock()
		return s.root, ctx.Err()
	}

	// Mark complete
	s.mu.Lock()
	s.progress.Complete = true
	s.mu.Unlock()

	if progressChan != nil {
		// Send final progress update
		select {
		case progressChan <- *s.progress:
		default:
		}
		close(progressChan)
	}

	return s.root, nil
}

// scanDirectoryParallel scans directories in parallel (up to depth 2)
func (s *Scanner) scanDirectoryParallel(ctx context.Context, node *FileNode, progressChan chan<- ScanProgress, depth int) {
	// Check if cancelled before starting
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Acquire semaphore for this directory read (prevents too many concurrent I/O operations)
	s.workerSem <- struct{}{}

	entries, err := s.readDirWithTimeout(node.Path)

	// Release semaphore immediately after reading (before processing children)
	<-s.workerSem

	if err != nil {
		s.recordError(fmt.Errorf("cannot read directory %s: %w", node.Path, err))
		// Don't return - continue with what we have
		entries = []os.DirEntry{} // Empty, so we'll just add this node without children
	}

	// For shallow depths, scan subdirectories in parallel
	if depth < 2 {
		var wg sync.WaitGroup
		var childrenMu sync.Mutex

		for _, entry := range entries {
			// Check if cancelled in loop
			select {
			case <-ctx.Done():
				wg.Wait() // Wait for already-started goroutines
				return
			default:
			}

			entryName := entry.Name()
			fullPath := filepath.Join(node.Path, entryName)

			// Skip iCloud placeholder files
			if isICloudPlaceholder(entryName) {
				s.mu.Lock()
				s.progress.ICloudFilesSkipped++
				s.mu.Unlock()
				continue
			}

			// Update progress (throttled)
			s.updateProgress(fullPath, progressChan)

			// Check if we should skip this path (network volume check)
			if shouldSkip, reason := s.volumeChecker.ShouldSkipPath(fullPath); shouldSkip {
				s.volumesMu.Lock()
				s.skippedVolumes = append(s.skippedVolumes, fullPath+" ("+reason+")")
				s.volumesMu.Unlock()
				continue
			}

			info, err := entry.Info()
			if err != nil {
				s.recordError(fmt.Errorf("cannot stat %s: %w", fullPath, err))
				continue
			}

			// For directories, check filesystem boundary and duplicate inodes
			if info.IsDir() {
				// Check if we've already scanned this directory (handles firmlinks/aliases)
				devID, inode, err := getDeviceAndInode(fullPath)
				if err == nil {
					if s.hasSeenInode(devID, inode) {
						// Already scanned this directory (it's an alias/firmlink)
						s.volumesMu.Lock()
						s.skippedVolumes = append(s.skippedVolumes, fullPath+" (alias/firmlink)")
						s.volumesMu.Unlock()
						continue
					}
					s.markInodeSeen(devID, inode)
				}

				if shouldSkip, reason := s.shouldSkipFilesystemBoundary(fullPath); shouldSkip {
					s.volumesMu.Lock()
					s.skippedVolumes = append(s.skippedVolumes, fullPath+" ("+reason+")")
					s.volumesMu.Unlock()
					continue
				}
			}

			childNode := NewFileNode(fullPath, info.Size(), info.IsDir(), info.ModTime())

			childrenMu.Lock()
			node.AddChild(childNode)
			childrenMu.Unlock()

			if info.IsDir() {
				// Scan subdirectories in parallel
				// Note: No semaphore here - it's acquired inside scanDirectoryParallel
				wg.Add(1)
				go func(n *FileNode) {
					defer wg.Done()
					s.scanDirectoryParallel(ctx, n, progressChan, depth+1)
				}(childNode)
			}
		}

		wg.Wait()
	} else {
		// For deeper levels, use sequential scanning to avoid too many goroutines
		s.scanDirectorySequential(ctx, node, progressChan)
	}
}

// scanDirectorySequential scans a directory sequentially
func (s *Scanner) scanDirectorySequential(ctx context.Context, node *FileNode, progressChan chan<- ScanProgress) {
	// Check if cancelled before starting
	select {
	case <-ctx.Done():
		return
	default:
	}

	entries, err := s.readDirWithTimeout(node.Path)
	if err != nil {
		s.recordError(fmt.Errorf("cannot read directory %s: %w", node.Path, err))
		// Don't return - continue with what we have (empty list)
		entries = []os.DirEntry{}
	}

	for _, entry := range entries {
		// Check if cancelled in loop
		select {
		case <-ctx.Done():
			return
		default:
		}

		entryName := entry.Name()
		fullPath := filepath.Join(node.Path, entryName)

		// Skip iCloud placeholder files
		if isICloudPlaceholder(entryName) {
			s.mu.Lock()
			s.progress.ICloudFilesSkipped++
			s.mu.Unlock()
			continue
		}

		// Update progress (throttled)
		s.updateProgress(fullPath, progressChan)

		// Check if we should skip this path (network volume check)
		if shouldSkip, reason := s.volumeChecker.ShouldSkipPath(fullPath); shouldSkip {
			s.volumesMu.Lock()
			s.skippedVolumes = append(s.skippedVolumes, fullPath+" ("+reason+")")
			s.volumesMu.Unlock()
			continue
		}

		info, err := entry.Info()
		if err != nil {
			s.recordError(fmt.Errorf("cannot stat %s: %w", fullPath, err))
			continue
		}

		// For directories, check filesystem boundary and duplicate inodes
		if info.IsDir() {
			// Check if we've already scanned this directory (handles firmlinks/aliases)
			devID, inode, err := getDeviceAndInode(fullPath)
			if err == nil {
				if s.hasSeenInode(devID, inode) {
					// Already scanned this directory (it's an alias/firmlink)
					s.volumesMu.Lock()
					s.skippedVolumes = append(s.skippedVolumes, fullPath+" (alias/firmlink)")
					s.volumesMu.Unlock()
					continue
				}
				s.markInodeSeen(devID, inode)
			}

			if shouldSkip, reason := s.shouldSkipFilesystemBoundary(fullPath); shouldSkip {
				s.volumesMu.Lock()
				s.skippedVolumes = append(s.skippedVolumes, fullPath+" ("+reason+")")
				s.volumesMu.Unlock()
				continue
			}
		}

		childNode := NewFileNode(fullPath, info.Size(), info.IsDir(), info.ModTime())
		node.AddChild(childNode)

		// Recursively scan subdirectories (sequential)
		if info.IsDir() {
			s.scanDirectorySequential(ctx, childNode, progressChan)
		}
	}
}

// updateProgress updates the scan progress (throttled)
func (s *Scanner) updateProgress(currentPath string, progressChan chan<- ScanProgress) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.progress.CurrentPath = currentPath
	s.progress.FilesScanned++

	// Only send updates every 100ms to avoid overwhelming the UI
	now := time.Now().UnixMilli()
	if progressChan != nil && (now - s.lastProgressUpdate > 100 || s.progress.FilesScanned % 100 == 0) {
		s.lastProgressUpdate = now
		// Non-blocking send
		select {
		case progressChan <- *s.progress:
		default:
			// Skip if channel is full
		}
	}
}

// recordError records an error during scanning
func (s *Scanner) recordError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.progress.Errors = append(s.progress.Errors, err)
}

// readDirWithTimeout wraps os.ReadDir with a timeout
// Returns entries and error, with timeout error if operation takes too long
func (s *Scanner) readDirWithTimeout(path string) ([]os.DirEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dirReadTimeout)
	defer cancel()

	type result struct {
		entries []os.DirEntry
		err     error
	}

	resultChan := make(chan result, 1)

	go func() {
		entries, err := os.ReadDir(path)
		resultChan <- result{entries: entries, err: err}
	}()

	select {
	case res := <-resultChan:
		return res.entries, res.err
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout reading directory (>%v): %s", dirReadTimeout, path)
	}
}

// getDeviceID returns the device ID for a given path
func getDeviceID(path string) (uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("cannot get device ID for %s", path)
	}

	return uint64(stat.Dev), nil
}

// getDeviceAndInode returns both device ID and inode for a path
func getDeviceAndInode(path string) (uint64, uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, err
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("cannot get stat info for %s", path)
	}

	return uint64(stat.Dev), stat.Ino, nil
}

// hasSeenInode checks if we've already scanned this inode
func (s *Scanner) hasSeenInode(deviceID uint64, inode uint64) bool {
	s.seenInodesMu.Lock()
	defer s.seenInodesMu.Unlock()

	if inodeMap, exists := s.seenInodes[deviceID]; exists {
		return inodeMap[inode]
	}
	return false
}

// markInodeSeen marks an inode as seen
func (s *Scanner) markInodeSeen(deviceID uint64, inode uint64) {
	s.seenInodesMu.Lock()
	defer s.seenInodesMu.Unlock()

	if _, exists := s.seenInodes[deviceID]; !exists {
		s.seenInodes[deviceID] = make(map[uint64]bool)
	}
	s.seenInodes[deviceID][inode] = true
}

// isICloudPlaceholder checks if a filename is an iCloud placeholder file
func isICloudPlaceholder(name string) bool {
	// iCloud placeholder files have the format: .filename.icloud
	return strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".icloud")
}

// shouldSkipFilesystemBoundary checks if we should skip a path due to filesystem boundaries
func (s *Scanner) shouldSkipFilesystemBoundary(path string) (bool, string) {
	if !s.oneFilesystem {
		return false, ""
	}

	devID, err := getDeviceID(path)
	if err != nil {
		// If we can't get device ID, continue (but log error)
		return false, ""
	}

	if devID != s.startDeviceID {
		return true, fmt.Sprintf("different filesystem (device %d)", devID)
	}

	return false, ""
}

// GetProgress returns the current scan progress
func (s *Scanner) GetProgress() ScanProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.progress
}

// CalculateStats computes aggregate statistics for a file tree
func CalculateStats(root *FileNode) *DirStats {
	stats := &DirStats{
		LargestFiles:  make([]*FileNode, 0),
		TypeBreakdown: make(map[string]*TypeStats),
	}

	walkTree(root, stats)

	// Sort largest files
	sort.Slice(stats.LargestFiles, func(i, j int) bool {
		return stats.LargestFiles[i].TotalSize() > stats.LargestFiles[j].TotalSize()
	})

	// Keep only top 100
	if len(stats.LargestFiles) > 100 {
		stats.LargestFiles = stats.LargestFiles[:100]
	}

	return stats
}

// walkTree recursively walks the tree and collects statistics
func walkTree(node *FileNode, stats *DirStats) {
	if node.IsDir {
		stats.DirCount++
		for _, child := range node.Children {
			walkTree(child, stats)
		}
	} else {
		stats.FileCount++
		stats.TotalSize += node.Size

		// Track largest files
		stats.LargestFiles = append(stats.LargestFiles, node)

		// Track by type
		if typeStats, exists := stats.TypeBreakdown[node.FileType]; exists {
			typeStats.FileCount++
			typeStats.TotalSize += node.Size
			typeStats.Files = append(typeStats.Files, node)
		} else {
			stats.TypeBreakdown[node.FileType] = &TypeStats{
				Extension: node.FileType,
				FileCount: 1,
				TotalSize: node.Size,
				Files:     []*FileNode{node},
			}
		}
	}
}

// FlattenTree returns a flat list of all nodes (useful for sorting/filtering)
func FlattenTree(root *FileNode) []*FileNode {
	result := make([]*FileNode, 0)
	flattenRecursive(root, &result)
	return result
}

func flattenRecursive(node *FileNode, result *[]*FileNode) {
	*result = append(*result, node)
	for _, child := range node.Children {
		flattenRecursive(child, result)
	}
}
