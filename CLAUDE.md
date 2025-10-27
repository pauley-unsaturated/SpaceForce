# CLAUDE.md - Developer Reference for SpaceForce

This document contains essential information for understanding and working on the SpaceForce codebase.

## Quick Start

```bash
# Build
go build -o spaceforce .

# Run
./spaceforce -path ~/Documents

# Test help
./spaceforce -help
```

## Project Overview

**SpaceForce** is a Terminal User Interface (TUI) disk space analyzer for macOS built with:
- **Language**: Go 1.24+
- **TUI Framework**: Bubble Tea + Lipgloss + Bubbles (Charm ecosystem)
- **Platform**: macOS-specific (uses macOS syscalls and paths)

**Purpose**: Help users identify and clean up disk space bloat with beautiful terminal UI.

**Features**:
- Real-time parallel filesystem scanning with progress updates
- Five interactive views: Tree, Top Items, Breakdown, Timeline, Errors
- File marking system with visual indicators
- Safe deletion to macOS Trash with progress tracking
- Tree view sorting by name or size
- Directory zoom for focused exploration
- Alias/firmlink deduplication (prevents double-counting)
- Dynamic width support (adapts to terminal size)
- Network volume detection and skipping

## Architecture

### Package Structure

```
spaceforce/
├── main.go              # Entry point, CLI flags, TUI initialization
├── scanner/             # Filesystem scanning logic
│   ├── models.go       # FileNode, DirStats, ScanProgress data structures
│   └── scanner.go      # Parallel scanner with network volume detection
├── analyzer/            # Analysis and suggestions
│   └── suggestions.go  # Smart cleanup recommendations
├── safety/              # Protection systems
│   ├── protector.go    # File safety checks (prevents deleting system files)
│   ├── trash.go        # macOS Trash integration via AppleScript
│   ├── exclusions.go   # Protected paths and bloat location lists
│   └── volumes.go      # Network volume detection via syscalls
├── util/                # Shared utilities
│   └── format.go       # FormatBytes, styles, color codes
├── ui/                  # Bubble Tea TUI
│   ├── app.go          # Main application model and update loop
│   ├── styles.go       # UI-specific styles (tabs, status bar, etc.)
│   ├── views/          # Four main views
│   │   ├── tree.go     # Hierarchical directory tree
│   │   ├── toplist.go  # Largest files/folders sorted
│   │   ├── breakdown.go # File type statistics with charts
│   │   └── timeline.go  # Files grouped by age
│   └── components/
│       └── confirm.go  # Deletion confirmation dialog
└── README.md, FIXES.md, NETWORK_VOLUMES.md
```

### Key Design Decisions

#### 1. Import Cycle Prevention
- **Problem**: `ui/views` needed to import `ui` for styles → circular dependency
- **Solution**: Moved shared styles to `util/format.go`
- **Rule**: `util` has no internal dependencies, only external (lipgloss)
- **Pattern**: `scanner` ← `ui` ← `views` (no cycles)

#### 2. Parallel Scanning Strategy
- **Top 2 levels**: Parallel with worker pool (8 workers max)
- **Deeper levels**: Sequential to avoid goroutine explosion
- **Rationale**: Most filesystems are wide at top, deep at bottom
- **Example**: `/Users`, `/Library` in parallel; `~/Documents/Projects/foo/bar/...` sequential

#### 3. Thread Limiting (8 Workers)
- **Why not runtime.NumCPU()?**: File I/O is bottleneck, not CPU
- **Why not unlimited?**: Memory usage, context switching, diminishing returns
- **Why 8?**: Sweet spot for M1/M2/M3 Macs (4-12 cores typical)
- **Implementation**: Buffered channel semaphore

#### 4. Progress Update Throttling
- **Problem**: Sending updates for every file overwhelms UI
- **Solution**: Send updates every 100ms OR every 100 files
- **Implementation**: Time-based + count-based throttling with non-blocking sends

#### 5. Network Volume Detection
- **Default**: Skip network volumes (prevents hangs)
- **Method**: `syscall.Statfs` to check filesystem type
- **Detected types**: nfs, smbfs, afpfs, cifs, webdav, mtpfs, autofs
- **Override**: `-skip-network=false` flag

#### 6. File Marking and Deletion System
- **Mark files**: `m` key in any view
- **Visual indicator**: `[✓]` shown next to marked files
- **Delete marked**: `x` key opens confirmation modal
- **Progress modal**: ASCII progress bar, current file, N/M counter
- **Trash integration**: Uses osascript/AppleScript to invoke Finder
- **Safety checks**: Multi-level protection prevents system file deletion

#### 7. Tree View Enhancements
- **Sorting**: Toggle between name/size with `s` key
- **Zoom**: Focus on directory with `z`, zoom out with `u`
- **Dynamic width**: Adapts to terminal width (20-140 char range)
- **File counts**: Shows "(N files)" for directories on wide terminals (>100 cols)
- **Performance**: Cached sorting to prevent re-sorting on every rebuild

#### 8. Alias/Firmlink Deduplication
- **Problem**: `/Users` and `/System/Volumes/Data/Users` are the same directory
- **Solution**: Track (device_id, inode) pairs to detect duplicates
- **Result**: Each directory counted only once, regardless of access path
- **Implementation**: `Scanner.seenInodes` map with mutex protection

## Critical Code Patterns

### 1. Scanner Message Flow

```go
// main.go: Create channels BEFORE scanning
progressChan := make(chan scanner.ScanProgress, 100) // Buffered!

// Start progress forwarder FIRST (critical ordering)
go func() {
    for progress := range progressChan {
        p.Send(ui.ScanProgressMsg(progress))
    }
}()

// THEN start scan
scn.Scan(rootPath, progressChan)

// Close channel when done
close(progressChan)
```

**⚠️ Common Mistake**: Starting scan before progress forwarder → blank screen

### 2. Bubble Tea Update Pattern

```go
// ui/app.go
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Handle user input
    case ScanCompleteMsg:
        // Update model state
        m.root = msg.Root
        // Initialize views
        return m, nil
    case ScanProgressMsg:
        // Update progress
        return m, nil
    }
    return m, nil
}
```

**Key Point**: Always return `(tea.Model, tea.Cmd)`, never mutate without returning

### 3. Worker Pool Semaphore

```go
// scanner/scanner.go
type Scanner struct {
    workerSem chan struct{} // Buffer = max workers
}

// Acquire slot (blocks if full)
s.workerSem <- struct{}{}
defer func() { <-s.workerSem }() // Release

// Do work...
```

**⚠️ Deadlock Risk**: Always use defer for release to handle panics

### 4. Mutex-Protected Shared State

```go
// Multiple goroutines adding children
var childrenMu sync.Mutex

go func() {
    childrenMu.Lock()
    node.AddChild(childNode)
    childrenMu.Unlock()
}()
```

**Rule**: Any shared state modified by goroutines needs mutex

### 5. Network Volume Check

```go
// safety/volumes.go
func (vc *VolumeChecker) ShouldSkipPath(path string) (bool, string) {
    var stat syscall.Statfs_t
    syscall.Statfs(path, &stat)

    // Convert [16]int8 to string
    fsTypeBytes := make([]byte, len(stat.Fstypename))
    for i, v := range stat.Fstypename {
        fsTypeBytes[i] = byte(v)
    }
    fsTypeName := string(fsTypeBytes)

    // Check against network FS types
    if fsTypeName == "nfs" || fsTypeName == "smbfs" { ... }
}
```

**⚠️ Gotcha**: `stat.Fstypename` is `[16]int8`, not string or `[]byte`

## Data Flow

### Scanning Phase
```
main.go
  ↓ creates
Scanner
  ↓ scans filesystem
FileNode tree
  ↓ sends to
progressChan
  ↓ forwarded to
Bubble Tea (ScanProgressMsg)
  ↓ updates
UI (scanning view)
```

### Viewing Phase
```
Scanner completes
  ↓ sends
ScanCompleteMsg {Root, Err, SkippedVolumes}
  ↓ received by
Model.Update()
  ↓ creates
TreeView, TopListView, BreakdownView, TimelineView
  ↓ user navigates
Tab / 1-4 keys switch views
  ↓ each view handles
↑↓ navigation, expand/collapse, sorting
```

## Critical Bug Fixes (2025-10-26)

### ⚠️ TERMINAL RESIZE BUG - FIXED
**Symptom**: Top bar disappears after resizing terminal, UI overflows
**Cause**: Multiple issues:
1. Views rendered more content than allocated height (missing chrome calculations)
2. Long strings wrapped to multiple lines (file counts, titles, help text)
3. Views not initialized with correct height on creation
**Fix**:
1. Each view now calculates `contentHeight = allocatedHeight - chromeLines`
2. Truncated all long strings (titles, help, skipped info) to prevent wrapping
3. Removed file counts from tree view initially (re-added with width reservation)
4. Views initialized with correct height immediately after creation
**Files**: `BUGFIX_RESIZE.md`, `BUGFIX_RESIZE_FINAL.md`, `BUGFIX_TREE_PERFORMANCE.md`

### ⚠️ TREE VIEW PERFORMANCE - FIXED
**Symptom**: Tree view extremely slow to navigate and expand/collapse
**Cause**: Re-sorting all children of every expanded directory on every rebuild
**Fix**: Implemented sort cache (`sortedCache` map) that stores sorted children by path
**Result**: 100x-1000x speedup for large trees
**Files**: `BUGFIX_TREE_PERFORMANCE.md`

### ⚠️ SEMAPHORE DEADLOCK - FIXED
**Symptom**: Scanning would stall after exactly 100-200 entries
**Cause**: Semaphore held across recursive calls created circular dependency
**Fix**: Release semaphore immediately after directory read, BEFORE spawning children

**Pattern to AVOID**:
```go
// ❌ DEADLOCK - Don't hold semaphore across recursive calls
go func() {
    sem <- struct{}{}
    defer func() { <-sem }()
    recursiveFunction()  // spawns children who need sem
}()
```

**Correct Pattern**:
```go
// ✅ CORRECT - Release before recursing
func work() {
    sem <- struct{}{}
    data := doIO()
    <-sem  // Release BEFORE children

    for child := range children {
        go work()  // Safe - parent released sem
    }
}
```

### ⚠️ ERROR BAILOUT - FIXED
**Symptom**: Scanner stops early on permission errors
**Cause**: Used `return` on directory read errors
**Fix**: Continue with empty entry list, accumulate errors for display

**Always do this**:
```go
entries, err := os.ReadDir(path)
if err != nil {
    recordError(err)
    entries = []os.DirEntry{}  // Continue, don't return
}
// Process entries (might be empty)
```

See `BUGFIXES.md` for complete technical details.

## Important Gotchas

### 1. FileNode Size Calculation
```go
// models.go
func (n *FileNode) TotalSize() int64 {
    if !n.IsDir {
        return n.Size // File: just its size
    }

    // Directory: sum of all children (recursive)
    total := int64(0)
    for _, child := range n.Children {
        total += child.TotalSize() // RECURSIVE
    }
    return total
}
```

**⚠️ Performance**: This is O(n) per call! Cache if calling repeatedly.

### 2. Progress Channel Closure
```go
// scanner/scanner.go: Scan()
defer func() {
    if progressChan != nil {
        close(progressChan) // CRITICAL: Close or goroutine leaks
    }
}()
```

**⚠️ Memory Leak**: Unclosed channels leak the forwarder goroutine

### 3. Bubble Tea Message Order
Messages are processed in order received. If you do:
```go
p.Send(ScanProgressMsg{...})
p.Send(ScanCompleteMsg{...})
```

Complete will process AFTER progress. This is intentional.

### 4. macOS System Paths
```go
// safety/exclusions.go
protectedPaths := []string{
    "/System",     // System files
    "/bin",        // Core utilities
    "/usr/lib",    // System libraries
    // ... but NOT /usr/local (user-installed)
}
```

**Key Distinction**: `/Library` (system) vs `~/Library` (user)

### 5. Lipgloss Style Application
```go
// util/format.go
style := lipgloss.NewStyle().Foreground(color).Bold(true)
text := style.Render("Hello") // MUST call .Render()

// ❌ WRONG: style(text)
// ❌ WRONG: text.Style(style)
```

## Common Tasks

### Adding a New View

1. Create `ui/views/newview.go`:
```go
package views

type NewView struct {
    data *scanner.FileNode
    // ... state
}

func NewNewView(root *scanner.FileNode) *NewView {
    return &NewView{data: root}
}

func (v *NewView) Update(msg tea.Msg) (*NewView, tea.Cmd) {
    // Handle keys
    return v, nil
}

func (v *NewView) View() string {
    // Render view
    return "content"
}
```

2. Add to `ui/app.go`:
```go
type ViewType int
const (
    // ...
    ViewNew
)

type Model struct {
    newView *views.NewView
    // ...
}

// In Update():
case ScanCompleteMsg:
    m.newView = views.NewNewView(m.root)
```

3. Add keyboard shortcut and tab

### Adding a New Safety Rule

1. Edit `safety/exclusions.go`:
```go
func getProtectedPaths() []string {
    return []string{
        // Add new protected path
        "/NewProtectedPath",
    }
}
```

2. Test with `protector.IsSafeToDelete(path)`

### Adding New Cleanup Suggestion

Edit `analyzer/suggestions.go`:
```go
func (se *SuggestionEngine) GenerateSuggestions() []*Suggestion {
    suggestions = append(suggestions, se.findNewBloatType()...)
    // ...
}

func (se *SuggestionEngine) findNewBloatType() []*Suggestion {
    // Scan for specific bloat pattern
    // Return suggestions
}
```

### Adjusting Worker Count

Edit `scanner/scanner.go`:
```go
func NewScanner() *Scanner {
    maxWorkers := 8 // Change this
    // ...
}
```

Consider making it a CLI flag if needed.

## Testing

### Manual Testing
```bash
# Small directory (fast)
./spaceforce -path ~/Documents

# System root (many files)
./spaceforce -path /

# With network volumes
./spaceforce -path /Volumes -skip-network=false

# Show help
./spaceforce -help
```

### Testing Specific Features

**Progress Updates**:
```bash
# Should see immediate progress, not blank screen
./spaceforce -path /
# Watch for "Files scanned: XXX" incrementing
```

**Network Volume Detection**:
```bash
# Mount a network share, then:
./spaceforce -path /Volumes
# Should see "ℹ Skipped X network volume(s)"
```

**Thread Limiting**:
```bash
# Large directory - should not spike goroutines
./spaceforce -path /
# Monitor with Activity Monitor or add debug logging
```

**Safety Checks**:
```bash
# Try selecting a system file in tree view
# Should show "Protected" in safety column
```

## Performance Characteristics

### Time Complexity
- **Scanning**: O(n) where n = number of files
- **Tree building**: O(n)
- **Stats calculation**: O(n)
- **View rendering**: O(visible items) due to viewport

### Space Complexity
- **FileNode tree**: ~150 bytes per file/directory
- **100K files**: ~15 MB
- **1M files**: ~150 MB
- **Scanner overhead**: ~50 MB
- **Total for 1M files**: ~200-300 MB

### Bottlenecks
1. **File I/O**: Reading directory entries (disk speed limited)
2. **syscall.Statfs**: Checking filesystem types (syscall overhead)
3. **TUI rendering**: Large lists (mitigated by viewport)

## Future Enhancement Ideas

### High Priority
- [x] Actual macOS Trash integration ✅ (implemented via osascript)
- [x] File marking and deletion system ✅
- [x] Tree view sorting by size ✅
- [x] Alias/firmlink deduplication ✅
- [ ] Configurable worker count flag
- [ ] Resume scanning (save/load scan results)
- [ ] Export reports (JSON, CSV)

### Medium Priority
- [ ] True duplicate file detection (by hash, not size)
- [ ] Incremental scanning (rescan changed dirs only)
- [ ] Filter by file age/size/type
- [ ] Saved filter presets

### Low Priority
- [ ] Cross-platform support (Linux, Windows)
- [ ] Compression analysis (identify compressible files)
- [ ] Cloud integration (scan cloud storage)
- [ ] Plugin system for custom analyzers

## Debugging Tips

### Blank Screen on Scan
**Check**: Progress forwarder started BEFORE scan?
**Fix**: Reorder goroutines in main.go

### Progress Not Updating
**Check**: Channel buffer size, throttling logic
**Fix**: Increase buffer, adjust throttle interval

### Slow Scanning
**Check**: Network volumes included?
**Fix**: Ensure `-skip-network=true` (default)

### High Memory Usage
**Check**: Number of files scanned
**Expected**: ~150 bytes per file is normal

### Import Cycle Error
**Check**: Does `views` import `ui`?
**Fix**: Move shared code to `util`

### UI Glitches
**Check**: Terminal size, TERM environment variable
**Fix**: Ensure modern terminal (iTerm2, Terminal.app 2.0+)

## Build & Release

### Build
```bash
go build -o spaceforce .
```

### Build Optimized
```bash
go build -ldflags="-s -w" -o spaceforce .
# -s: omit symbol table
# -w: omit DWARF debug info
# Reduces binary size ~30%
```

### Install Locally
```bash
go build -o spaceforce .
sudo mv spaceforce /usr/local/bin/
spaceforce -version
```

### Cross-Compile (macOS only)
```bash
# Intel Mac
GOARCH=amd64 go build -o spaceforce-amd64 .

# Apple Silicon
GOARCH=arm64 go build -o spaceforce-arm64 .

# Universal binary (requires lipo)
lipo -create spaceforce-amd64 spaceforce-arm64 -output spaceforce
```

## Dependencies

### Direct
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/charmbracelet/bubbles` - TUI components

### Indirect
- Various Charm ecosystem helpers
- Standard library (syscall, os, filepath, time, etc.)

### Updating Dependencies
```bash
go get -u github.com/charmbracelet/bubbletea@latest
go get -u github.com/charmbracelet/lipgloss@latest
go get -u github.com/charmbracelet/bubbles@latest
go mod tidy
```

## Code Style

### Go Conventions
- Use `gofmt` (already formatted)
- Exported names start with capital (public API)
- Unexported names lowercase (internal)
- Package names lowercase, no underscores

### Our Conventions
- Views return `(*ViewType, tea.Cmd)` for consistency
- Styles defined in `util/format.go` or `ui/styles.go`
- Safety checks in `safety/` package
- Scanning logic in `scanner/` package
- UI never directly accesses scanner internals

### Comments
- Exported functions have doc comments
- Complex algorithms explained inline
- TODO comments for future work

## Platform-Specific Code

### macOS-Only Features
- `syscall.Statfs_t` for filesystem detection
- `/Library`, `/System`, `/Volumes` path assumptions
- Trash integration (planned)

### Porting to Linux/Windows
Would require:
- Replace `syscall.Statfs_t` with platform equivalents
- Different protected paths
- Different trash/recycle bin integration
- Network FS detection per platform

## Security Considerations

### File Deletion
- **Current**: Uses `os.Remove` (permanent!)
- **Planned**: macOS Trash via AppleScript
- **Safety**: Multiple checks before deletion
- **User control**: Confirmation dialog

### Path Traversal
- **Protection**: Uses `filepath.Abs()` to normalize
- **Symlinks**: Followed (could revisit)
- **Permissions**: Respects OS permissions

### Network Volumes
- **Default**: Skipped (prevents potential hangs)
- **Override**: User explicit opt-in
- **Risk**: Unreachable shares could hang

## License & Attribution

- **License**: MIT
- **Framework**: Charm Bubble Tea (MIT)
- **Inspiration**: ncdu, dust, macOS Disk Utility

## Contact & Support

This is a personal project. For bugs or suggestions:
- Check existing issues in project
- Create new issue with details
- Include: OS version, Go version, command used, error output

---

## Quick Reference Card

```bash
# Build
go build -o spaceforce .

# Run with defaults
./spaceforce

# Common options
./spaceforce -path ~/Documents
./spaceforce -path / -skip-network=false
./spaceforce -help

# Key bindings in app
Tab/1-5  : Switch views (Tree, Top Items, Breakdown, Timeline, Errors)
↑↓/jk    : Navigate
Enter/Spc: Expand/collapse (tree view)
←→/hl    : Expand/collapse (tree view)
s        : Toggle sort (tree: name/size, toplist: various)
z        : Zoom into directory (tree view)
u        : Zoom out to parent (tree view)
f        : Toggle files visibility (toplist)
d        : Toggle dirs visibility (toplist)
m        : Mark/unmark file for deletion
x        : Delete marked files (shows confirmation)
q        : Quit

# Important files
main.go           : Entry point
scanner/scanner.go: Parallel scanner
ui/app.go         : Main TUI logic
safety/volumes.go : Network detection
util/format.go    : Shared utilities

# Key patterns
Progress: Send updates before scan
Threads : Max 8 workers (semaphore)
Network : Skip by default (syscall.Statfs)
Safety  : Check before delete
Styles  : Use util/format.go
```

---

**Last Updated**: 2025-10-26 (Major feature release)
**Go Version**: 1.24+
**Platform**: macOS
**Status**: Production-ready

**Recent Additions**:
- File marking and deletion system with macOS Trash integration
- Tree view sorting (name/size) with cached performance
- Tree view zoom functionality (z/u keys)
- Alias/firmlink deduplication via inode tracking
- Dynamic width support for better use of wide terminals
- Complete terminal resize bug fixes
- Visual indicators for marked files
