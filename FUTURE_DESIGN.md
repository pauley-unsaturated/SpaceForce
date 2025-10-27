# SpaceForce - Future Design Considerations

This document outlines future design improvements for SpaceForce to handle complex scanning scenarios and improve performance through caching and better data structures.

---

## 1. APFS Volume Container Detection

### Problem
On modern macOS systems with APFS, multiple volumes can share the same physical container. Currently, SpaceForce treats these as separate filesystems and won't scan across them with `--one-filesystem=true`.

**Example:**
```
Container disk1
├── Macintosh HD (/)
├── Macintosh HD - Data (/System/Volumes/Data)
└── Preboot, Recovery, etc.
```

### Desired Behavior
When scanning with `--one-filesystem=true`, we should:
1. Detect when volumes share the same APFS container
2. Allow scanning across volumes in the same container
3. Track and report disk usage **by volume** within the container
4. Show breakdown: "Total: 500GB (Macintosh HD: 300GB, Data: 200GB)"

### Implementation Approach

#### A. Detecting APFS Containers

**Method 1: Using `diskutil` command**
```bash
diskutil info /dev/disk1s1 | grep "APFS Volume Group"
# or
diskutil apfs list
```

**Method 2: Using IOKit (lower-level, more reliable)**
- Use `syscall.Statfs` to get `f_mntfromname` (e.g., "/dev/disk1s1")
- Parse the device identifier to extract container ID
- Query IOKit for container information

**Go Implementation:**
```go
type APFSContainer struct {
    ContainerID string
    Volumes     []VolumeInfo
}

type VolumeInfo struct {
    DeviceID  uint64
    MountPath string
    FSType    string
    Size      int64
}

// GetAPFSContainer determines if volumes share a container
func GetAPFSContainer(devicePath string) (*APFSContainer, error) {
    // Parse device path: /dev/disk1s1 -> disk1
    // Query container info using diskutil or IOKit
    // Return container with all its volumes
}
```

#### B. Modified Scanning Logic

**Current:**
```go
if devID != s.startDeviceID {
    return true, "different filesystem"
}
```

**Proposed:**
```go
if devID != s.startDeviceID {
    // Check if new device is in same APFS container
    if s.apfsContainer != nil {
        if s.apfsContainer.ContainsDevice(devID) {
            // Same container, different volume - continue scanning
            // Track which volume we're in
            s.trackVolumeTransition(devID)
            return false, ""
        }
    }
    return true, "different filesystem"
}
```

#### C. Volume-Aware Reporting

Track statistics per volume:
```go
type ContainerStats struct {
    TotalSize    int64
    VolumeBreakdown map[string]*VolumeStats
}

type VolumeStats struct {
    VolumeName   string
    DeviceID     uint64
    FilesScanned int64
    TotalSize    int64
    Directories  int64
}
```

**UI Enhancement:**
```
Total: 512 GB across 2 volumes
├── Macintosh HD: 300 GB (58%)
└── Data:         212 GB (42%)
```

### Challenges
- Requires elevated permissions to query some IOKit information
- Cross-volume symlinks and hard links need special handling
- Performance impact of tracking volume transitions
- Need to handle non-APFS filesystems gracefully (fall back to current behavior)

---

## 2. Data Structure Optimization

### Current Structure: In-Memory Tree

**Current Implementation:**
```go
type FileNode struct {
    Path     string
    Size     int64
    IsDir    bool
    Children []*FileNode
    Parent   *FileNode
    // ... more fields
}
```

**Problems:**
- High memory usage for large scans (1M+ files)
- No persistence - must re-scan every time
- Recursive operations can cause stack overflow
- Difficult to incrementally update

### Proposed: Hybrid Approach

#### A. Database-Backed Storage (SQLite)

**Schema:**
```sql
CREATE TABLE scans (
    id INTEGER PRIMARY KEY,
    root_path TEXT NOT NULL,
    scan_time INTEGER NOT NULL,
    device_id INTEGER,
    total_files INTEGER,
    total_size INTEGER
);

CREATE TABLE nodes (
    id INTEGER PRIMARY KEY,
    scan_id INTEGER,
    parent_id INTEGER,
    path TEXT NOT NULL,
    name TEXT NOT NULL,
    size INTEGER,
    is_dir BOOLEAN,
    mod_time INTEGER,
    file_type TEXT,
    device_id INTEGER,
    FOREIGN KEY(scan_id) REFERENCES scans(id),
    FOREIGN KEY(parent_id) REFERENCES nodes(id)
);

CREATE INDEX idx_nodes_scan ON nodes(scan_id);
CREATE INDEX idx_nodes_parent ON nodes(parent_id);
CREATE INDEX idx_nodes_path ON nodes(path);
CREATE INDEX idx_nodes_size ON nodes(size DESC);
```

**Benefits:**
- Persistent storage - no need to re-scan
- Can handle millions of files
- SQL queries for filtering, sorting, searching
- Incremental updates possible
- Lower memory footprint

**Drawbacks:**
- More complex code
- Slower than pure in-memory for small scans
- Requires SQLite dependency
- Need to handle database corruption

#### B. Alternative: Memory-Mapped Files

Use memory-mapped files for large data sets:
```go
type FileNodeCompact struct {
    PathOffset uint64  // Offset in string table
    Size       int64
    ModTime    int64
    ParentID   uint32
    Flags      uint16  // IsDir, FileType enum, etc.
}
```

**Benefits:**
- Fast access
- OS handles memory management
- Can exceed RAM capacity
- Simple implementation

**Drawbacks:**
- Fixed-size records
- String table management complexity
- Platform-specific code
- Difficult to update incrementally

#### C. Recommended: SQLite + In-Memory Cache

**Hybrid approach:**
1. Store full scan data in SQLite
2. Load hot data (top 1000 files, current view) into memory
3. Use lazy loading for tree traversal
4. Background worker updates database

**Implementation:**
```go
type ScanCache struct {
    db          *sql.DB
    scanID      int64
    rootNode    *FileNode
    topFiles    []*FileNode  // Top 100 largest
    typeStats   map[string]*TypeStats
    recentPaths map[string]*FileNode // LRU cache
}

func (sc *ScanCache) LoadNode(path string) (*FileNode, error) {
    // Check cache first
    if node, ok := sc.recentPaths[path]; ok {
        return node, nil
    }

    // Load from database
    node, err := sc.loadFromDB(path)
    if err != nil {
        return nil, err
    }

    // Add to cache
    sc.recentPaths[path] = node
    return node, nil
}
```

---

## 3. Incremental Scanning

### Problem
Re-scanning large directories takes too long. Need ability to:
- Detect what has changed since last scan
- Update only modified parts of the tree
- Show diff: "500 MB added, 200 MB removed"

### Approach

#### A. Metadata Comparison
```go
type ScanMetadata struct {
    Path      string
    ModTime   time.Time
    Size      int64
    FileCount int64
    Checksum  string // Fast hash of directory listing
}
```

**Algorithm:**
1. Load previous scan metadata from database
2. For each directory, compare:
   - Modification time
   - File count
   - Quick checksum of `ls` output
3. If unchanged, skip recursion
4. If changed, re-scan only that subtree

**Performance:**
- First scan: Full (1M files, 10 minutes)
- Incremental scan: Fast (100 changed directories, 30 seconds)

#### B. Filesystem Events (Future)
Use FSEvents API (macOS) to track changes in real-time:
```go
watcher, _ := fsnotify.NewWatcher()
watcher.Add(rootPath)

for {
    select {
    case event := <-watcher.Events:
        // Update database incrementally
        handleFileChange(event)
    }
}
```

**Benefits:**
- Near-instant updates
- No re-scanning needed
- Live monitoring

**Challenges:**
- High memory usage
- Difficult to get initial state
- Reliability issues with many files

---

## 4. Performance Optimizations

### A. Parallel Database Writes

Current: Single-threaded writes
```go
for _, node := range nodes {
    db.Insert(node)  // Slow!
}
```

Optimized: Batch inserts with workers
```go
ch := make(chan *FileNode, 1000)

// Workers
for i := 0; i < 4; i++ {
    go func() {
        batch := make([]*FileNode, 0, 100)
        for node := range ch {
            batch = append(batch, node)
            if len(batch) >= 100 {
                db.InsertBatch(batch)
                batch = batch[:0]
            }
        }
    }()
}

// Scanner feeds workers
scanner.Scan(path, ch)
```

### B. Streaming Processing

Don't load entire tree into memory:
```go
// Instead of:
root := scanner.Scan(path)
stats := calculateStats(root)  // Loads everything

// Do:
scanner.ScanStreaming(path, func(node *FileNode) {
    updateStats(node)  // Process on the fly
    db.Insert(node)
})
```

### C. Compression

Store paths efficiently:
```go
// Instead of: "/Users/mark/Documents/foo.txt" (29 bytes)
// Store: parent_id + "foo.txt" (4 bytes + 7 bytes = 11 bytes)

// Or use path compression:
// "/Users/mark/" -> ID 1
// ID 1 + "Documents/" -> ID 2
// ID 2 + "foo.txt" -> file node
```

---

## 5. Implementation Priority

### Phase 1 (Immediate - Current Release)
- ✅ Device ID tracking (du -x style)
- ✅ iCloud placeholder detection
- ✅ Filesystem boundary detection
- ✅ Basic skip mechanisms

### Phase 2 (Next Release)
- [ ] SQLite-backed storage
- [ ] Scan persistence
- [ ] Basic incremental scanning (mod time-based)
- [ ] Performance optimizations (batching, streaming)

### Phase 3 (Future)
- [ ] APFS container detection
- [ ] Volume-aware reporting
- [ ] Advanced incremental scanning
- [ ] Filesystem event monitoring

### Phase 4 (Wishlist)
- [ ] Network storage for scan data
- [ ] Multi-machine scanning
- [ ] Historical tracking ("files growing over time")
- [ ] AI-powered cleanup suggestions

---

## 6. Database Schema (Detailed)

### Full Proposed Schema
```sql
-- Scan metadata
CREATE TABLE scans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    root_path TEXT NOT NULL,
    scan_time INTEGER NOT NULL,  -- Unix timestamp
    device_id INTEGER,
    volume_name TEXT,
    container_id TEXT,
    total_files INTEGER,
    total_dirs INTEGER,
    total_size INTEGER,
    scan_duration_ms INTEGER,
    completed BOOLEAN DEFAULT 0,
    version TEXT  -- Schema version
);

-- File/directory nodes
CREATE TABLE nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scan_id INTEGER NOT NULL,
    parent_id INTEGER,
    path TEXT NOT NULL,
    name TEXT NOT NULL,
    size INTEGER NOT NULL,
    is_dir BOOLEAN NOT NULL,
    mod_time INTEGER,
    file_type TEXT,
    device_id INTEGER,
    inode INTEGER,
    is_protected BOOLEAN DEFAULT 0,
    FOREIGN KEY(scan_id) REFERENCES scans(id) ON DELETE CASCADE,
    FOREIGN KEY(parent_id) REFERENCES nodes(id) ON DELETE CASCADE
);

-- Pre-computed statistics
CREATE TABLE type_stats (
    scan_id INTEGER NOT NULL,
    file_type TEXT NOT NULL,
    file_count INTEGER NOT NULL,
    total_size INTEGER NOT NULL,
    PRIMARY KEY(scan_id, file_type),
    FOREIGN KEY(scan_id) REFERENCES scans(id) ON DELETE CASCADE
);

-- Errors encountered during scan
CREATE TABLE scan_errors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scan_id INTEGER NOT NULL,
    path TEXT NOT NULL,
    error_type TEXT NOT NULL,
    error_message TEXT,
    timestamp INTEGER,
    FOREIGN KEY(scan_id) REFERENCES scans(id) ON DELETE CASCADE
);

-- Volume information
CREATE TABLE volumes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scan_id INTEGER NOT NULL,
    device_id INTEGER NOT NULL,
    mount_path TEXT NOT NULL,
    volume_name TEXT,
    container_id TEXT,
    fs_type TEXT,
    total_size INTEGER,
    FOREIGN KEY(scan_id) REFERENCES scans(id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX idx_nodes_scan ON nodes(scan_id);
CREATE INDEX idx_nodes_parent ON nodes(parent_id);
CREATE INDEX idx_nodes_path ON nodes(path);
CREATE INDEX idx_nodes_size ON nodes(size DESC);
CREATE INDEX idx_nodes_modtime ON nodes(mod_time DESC);
CREATE INDEX idx_nodes_type ON nodes(file_type);
CREATE INDEX idx_type_stats_size ON type_stats(scan_id, total_size DESC);

-- Full-text search on paths (optional)
CREATE VIRTUAL TABLE nodes_fts USING fts5(
    path,
    name,
    content='nodes',
    content_rowid='id'
);
```

---

## 7. Configuration File Support

Add `~/.spaceforce/config.json`:
```json
{
  "scan_cache_dir": "~/.spaceforce/cache",
  "default_options": {
    "skip_network": true,
    "one_filesystem": true,
    "timeout_seconds": 5
  },
  "exclude_patterns": [
    "node_modules",
    ".git",
    "__pycache__"
  ],
  "ui": {
    "theme": "dark",
    "show_hidden": false
  }
}
```

---

## Summary

This document outlines several major improvements for SpaceForce:

1. **APFS Container Support** - Scan across volumes in same container
2. **Database Backend** - SQLite for persistence and large scans
3. **Incremental Scanning** - Only re-scan what changed
4. **Performance Optimizations** - Batching, streaming, compression

**Recommendation:** Implement these in phases, starting with database backend in Phase 2, as it provides the most immediate value (persistence) and enables other features (incremental scanning, better performance).

**Estimated Effort:**
- Phase 2 (Database): 2-3 weeks
- Phase 3 (APFS + Incremental): 3-4 weeks
- Phase 4 (Wishlist): 2-3 months

---

**Document Version:** 1.0
**Last Updated:** 2025-10-26
**Status:** Design Proposal
