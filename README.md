# 🚀 SpaceForce

A beautiful Terminal User Interface (TUI) application for analyzing and managing disk space on macOS.

## Features

- **📁 Tree View** - Navigate your filesystem in a hierarchical tree structure with sorting (name/size) and zoom capabilities
- **📊 Top Items** - See the largest files and folders sorted by size, name, or modification date
- **📈 File Type Breakdown** - Analyze space usage by file type with visual charts
- **⏰ Timeline View** - Find old files grouped by modification date
- **🗑️ Safe Deletion** - Mark files for deletion with visual indicators and strong confirmation dialogs
- **🛡️ Two-Tier Protection** - System files blocked absolutely, sensitive paths require double confirmation
- **🚫 Root Safety** - Prevents running as root/sudo to avoid catastrophic system damage
- **📊 Progress Tracking** - Real-time byte-based progress bar during filesystem scanning
- **🌐 Network Volume Detection** - Automatically skips network volumes to prevent hangs
- **🔗 Alias Deduplication** - Prevents double-counting firmlinks and aliases via inode tracking
- **💡 Smart Suggestions** - Automated detection of common bloat locations (caches, build artifacts, etc.)
- **🎨 Beautiful UI** - Built with the Charm Bubble Tea ecosystem for a delightful terminal experience

## Installation

### Prerequisites

- Go 1.21 or later (Go 1.24+ recommended)
- macOS 10.15 or later (Catalina+)
- Terminal with modern Unicode support (iTerm2, Terminal.app, etc.)

### Build from Source

```bash
# Clone or navigate to the project directory
cd SpaceForce

# Install dependencies
go mod download

# Build the application
go build -o spaceforce .

# Optionally, move to your PATH
sudo mv spaceforce /usr/local/bin/
```

## Usage

### Basic Usage

```bash
# Scan current directory
./spaceforce

# Scan your home directory
./spaceforce -path ~

# Scan a specific directory
./spaceforce -path /Users/yourname/Downloads

# Include network volumes (skipped by default)
./spaceforce -path /Volumes -skip-network=false

# Allow crossing filesystem boundaries
./spaceforce -path / -one-filesystem=false
```

### Command-Line Flags

- `-path <directory>` - Directory to scan (default: current directory)
- `-skip-network` - Skip network volumes to prevent hangs (default: true)
- `-one-filesystem` - Stay on one filesystem like `du -x` (default: true)
- `-version` - Show version information
- `-help` - Show help message

### Keyboard Controls

#### Navigation
- `Tab` / `Shift+Tab` - Switch between views (forward/backward)
- `1` - Jump to Tree View
- `2` - Jump to Top Items View
- `3` - Jump to File Type Breakdown
- `4` - Jump to Timeline View
- `5` - Jump to Errors View
- `↑/↓` or `j/k` - Navigate up/down
- `q` - Quit

#### Tree View
- `Enter` or `Space` - Expand/collapse directory
- `→` or `l` - Expand directory
- `←` or `h` - Collapse directory
- `s` - Toggle sort mode (name ↔ size)
- `z` - Zoom into selected directory
- `u` - Zoom out to parent directory
- `m` - Mark/unmark file for deletion
- `x` - Delete marked files (with confirmation)

#### Top Items View
- `s` - Cycle sort mode (size → name → modified)
- `f` - Toggle files visibility
- `d` - Toggle directories visibility
- `Enter` - Jump to selected item in Tree View
- `m` - Mark/unmark file for deletion
- `x` - Delete marked files (with confirmation)

## Architecture

```
SpaceForce/
├── main.go                 # Entry point
├── scanner/
│   ├── scanner.go         # Filesystem scanning logic
│   └── models.go          # Data structures
├── analyzer/
│   └── suggestions.go     # Cleanup recommendations
├── safety/
│   ├── protector.go       # Two-tier protection system
│   ├── exclusions.go      # Protected and sensitive paths
│   ├── trash.go           # Deletion operations
│   └── volumes.go         # Network volume detection
├── util/
│   └── format.go          # Formatting & shared styles
├── ui/
│   ├── app.go            # Main Bubble Tea app model
│   ├── styles.go         # UI-specific styles
│   ├── views/
│   │   ├── tree.go       # Tree view component
│   │   ├── toplist.go    # Top files/folders view
│   │   ├── breakdown.go  # File type breakdown view
│   │   └── timeline.go   # Timeline view
│   └── components/
│       └── confirm.go    # Deletion confirmation dialog
└── go.mod
```

## Safety Features

SpaceForce includes multiple layers of protection to prevent accidental data loss:

### Root Safety Check
- **Blocks execution as root/sudo** - Running as root bypasses permission checks and could allow deletion of critical system files
- Clear warning displayed if attempted
- Prevents catastrophic system damage from accidental deletions

### Two-Tier Protection System

#### Tier 1: Absolutely Protected (Cannot Delete)
These paths are **completely blocked** from deletion:
- System directories (`/System`, `/bin`, `/sbin`, `/usr/lib`, `/usr/libexec`, etc.)
- System-level Library (`/Library/*`)
- Core system paths (`/etc`, `/dev`, `/cores`, `/private/etc`, `/private/var/db`)
- macOS system applications (`/System/Applications/*`)
- System frameworks and kernel extensions (`.kext`, `.dylib`, `.framework` in system paths)
- Boot volumes (`/Volumes/Macintosh HD`, `/Volumes/Recovery`)

#### Tier 2: Sensitive Paths (Require Double Confirmation)
These paths can be deleted but require typing `Y` **twice**:
- User Library directory (`~/Library/*`)
- Application data and preferences (`~/Library/Application Support`, `~/Library/Preferences`)
- Sandboxed app data (`~/Library/Containers`, `~/Library/Group Containers`)
- Credential directories (`~/.ssh`, `~/.gnupg`, `~/.aws`, `~/.kube`, `~/.docker`)
- Important user folders (`~/Documents`, `~/Desktop`)
- User home directory itself (`~`)

#### Regular Files (Single Confirmation)
All other user files (e.g., `~/Downloads`, `~/Pictures`) require one `Y` confirmation.

### Deletion Method
**Permanent deletion using `os.RemoveAll()`**
- Files are **permanently deleted**, not moved to Trash
- Strong confirmation dialogs compensate for permanent deletion
- Shows tree preview of what will be deleted before confirmation
- After deletion, views update to show reclaimed space
- All deleted items are removed from the tree in real-time

### Risk Level Indicators
Files display color-coded safety levels in the UI:
- **🟢 Safe (0)** - User caches, build artifacts, temporary files
- **🟡 Low Risk (1)** - User documents, downloads (in known safe directories)
- **🟠 Medium Risk (2)** - Read-only or unknown location files
- **🔴 Protected (3)** - System files, completely blocked from deletion

### Deletion Workflow

SpaceForce uses a safe, multi-step deletion process:

1. **Mark files** - Press `m` on any file/directory to mark it (shows `[✓]` indicator)
2. **Review selection** - Marked files persist across views, review in Tree or Top Items
3. **Initiate deletion** - Press `x` to open confirmation dialog
4. **Preview** - Dialog shows tree view of exactly what will be deleted
5. **Confirm** - Type `Y` to confirm (or `YY` for sensitive paths)
6. **Progress** - Watch real-time progress with file names and progress bar
7. **Summary** - See total files deleted, space reclaimed, and any errors
8. **Update** - Tree and views automatically update to reflect remaining files

## Smart Cleanup Suggestions

SpaceForce automatically identifies common sources of disk bloat:

- **Development**
  - Xcode DerivedData and Archives
  - Docker containers and images
  - npm, cargo, and other package manager caches
  - Build artifacts (node_modules, target/, build/, dist/)

- **Applications**
  - Application caches (`~/Library/Caches`)
  - Browser caches
  - Log files
  - Temporary files

- **System**
  - Homebrew package cache
  - Old system logs
  - Crash reports

## Technical Details

### Built With
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Style definitions for terminal UIs
- [Bubbles](https://github.com/charmbracelet/bubbles) - Common TUI components

### Performance
- **Parallel scanning** - Top-level directories scanned in parallel with worker pool (8 workers)
- **Real-time progress** - Byte-based progress bar with file count and current file display
- **Network volume detection** - Automatically skips network filesystems to prevent hangs
- **Alias deduplication** - Uses inode tracking to prevent double-counting firmlinks and aliases
- **Cached sorting** - Tree view caches sorted children for instant navigation
- **Lazy tree expansion** - Only renders visible items in viewport
- **Efficient updates** - After deletion, only affected tree nodes are rebuilt

## Known Limitations

- **macOS-specific** - Safety rules and paths are macOS-centric (would need modification for Linux/Windows)
- **Large scans** - Directories with 1M+ files may take time to scan (progress bar shows real-time status)
- **Permanent deletion** - Files are permanently deleted, not moved to Trash (strong confirmations compensate)
- **No undo** - Once deleted, files cannot be recovered (always review carefully before confirming)

## Future Enhancements

- [ ] Duplicate file detection (by content hash, not just size)
- [ ] Export reports (JSON, CSV, HTML)
- [ ] Saved scan sessions (resume analysis later)
- [ ] Configuration file support (customize protected paths, risk levels)
- [ ] Cross-platform support (Linux, Windows with platform-specific safety rules)
- [ ] Dry-run mode (preview what would be deleted without actually deleting)
- [ ] File preview in UI (quick look at file contents)
- [ ] Search/filter functionality (find files by name, type, size, date)

## Contributing

This is a personal project, but suggestions and bug reports are welcome!

## License

MIT License - Feel free to use and modify as needed.

## Acknowledgments

- Built with the amazing [Charm](https://charm.sh) ecosystem
- Inspired by tools like `ncdu`, `dust`, and macOS's Disk Utility
- Created as a learning project for Go TUI development

---

## ⚠️ IMPORTANT SAFETY WARNINGS

- **Files are PERMANENTLY DELETED** - Not moved to Trash, cannot be recovered
- **No undo** - Once you confirm deletion, files are immediately removed from disk
- **Review carefully** - Always double-check what you're deleting before confirming
- **When in doubt, don't delete** - If you're unsure, back up first or skip the file
- **Never run as root** - SpaceForce blocks sudo/root to prevent system damage
- **System files are protected** - But user files in `~/Downloads`, etc. are not

**You are ultimately responsible for what you delete.** SpaceForce provides strong protections and confirmations, but cannot prevent user error. Always review before confirming deletion.
