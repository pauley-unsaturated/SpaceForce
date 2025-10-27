# 🚀 SpaceForce

A beautiful Terminal User Interface (TUI) application for analyzing and managing disk space on macOS.

## Features

- **📁 Tree View** - Navigate your filesystem in a hierarchical tree structure
- **📊 Top Items** - See the largest files and folders sorted by size
- **📈 File Type Breakdown** - Analyze space usage by file type with visual charts
- **⏰ Timeline View** - Find old files grouped by modification date
- **🛡️ Safety Protection** - Intelligent checks prevent deletion of system-critical files
- **💡 Smart Suggestions** - Automated detection of common bloat locations (caches, build artifacts, etc.)
- **🎨 Beautiful UI** - Built with the Charm Bubble Tea ecosystem for a delightful terminal experience

## Installation

### Prerequisites

- Go 1.21 or later
- macOS (tested on macOS 10.15+)

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
```

### Keyboard Controls

#### Navigation
- `Tab` - Switch between views
- `1` - Jump to Tree View
- `2` - Jump to Top Items View
- `3` - Jump to File Type Breakdown
- `4` - Jump to Timeline View
- `↑/↓` or `j/k` - Navigate up/down
- `q` - Quit

#### Tree View
- `Enter` or `Space` - Expand/collapse directory
- `→` or `l` - Expand directory
- `←` or `h` - Collapse directory

#### Top Items View
- `s` - Cycle sort mode (size → name → modified)
- `f` - Toggle files visibility
- `d` - Toggle directories visibility

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
│   ├── protector.go       # System file detection
│   └── exclusions.go      # Protected paths list
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

### Protected Locations
- System directories (`/System`, `/bin`, `/usr/lib`, etc.)
- macOS frameworks and extensions
- Application bundles in `/System/Applications`
- Critical user data (keychains, SSH keys, preferences)

### Risk Levels
Files and directories are categorized by deletion risk:
- **0 (Safe)** - User caches, build artifacts, temporary files
- **1 (Low Risk)** - User documents, downloads (review recommended)
- **2 (Medium Risk)** - Application data (careful review needed)
- **3 (Protected)** - System files, blocked from deletion

### Deletion Method
All deletions move files to macOS Trash, allowing recovery if needed.

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
- Efficient filesystem scanning with progress tracking
- Lazy tree expansion for large directories
- Viewport rendering for smooth navigation of large file lists

## Known Limitations

- macOS-specific safety rules (not suitable for Linux/Windows without modification)
- Large directory scans (1M+ files) may take time
- Deletion uses `os.Remove` (trash integration requires additional setup)

## Future Enhancements

- [ ] Actual macOS Trash integration via AppleScript
- [ ] Duplicate file detection (by hash, not just size)
- [ ] Export reports (JSON, CSV)
- [ ] Interactive deletion mode
- [ ] Saved scan sessions
- [ ] Configuration file support
- [ ] Cross-platform support (Linux, Windows)

## Contributing

This is a personal project, but suggestions and bug reports are welcome!

## License

MIT License - Feel free to use and modify as needed.

## Acknowledgments

- Built with the amazing [Charm](https://charm.sh) ecosystem
- Inspired by tools like `ncdu`, `dust`, and macOS's Disk Utility
- Created as a learning project for Go TUI development

---

**⚠️ Important**: Always review files before deletion. While SpaceForce has safety protections, you are ultimately responsible for what you delete. When in doubt, back up first!
