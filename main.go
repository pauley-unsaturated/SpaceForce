package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"spaceforce/scanner"
	"spaceforce/ui"
)

var (
	version = "1.0.0"
)

func main() {
	// Parse command-line flags
	var (
		scanPath      = flag.String("path", ".", "Path to scan")
		skipNetwork   = flag.Bool("skip-network", true, "Skip network volumes (default: true)")
		oneFilesystem = flag.Bool("one-filesystem", true, "Stay on one filesystem (like du -x)")
		showVersion   = flag.Bool("version", false, "Show version")
		showHelp      = flag.Bool("help", false, "Show help")
	)

	flag.Parse()

	if *showVersion {
		fmt.Printf("SpaceForce v%s\n", version)
		os.Exit(0)
	}

	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	// Validate path
	if *scanPath == "" {
		fmt.Println("Error: path cannot be empty")
		os.Exit(1)
	}

	info, err := os.Stat(*scanPath)
	if err != nil {
		fmt.Printf("Error: cannot access path '%s': %v\n", *scanPath, err)
		os.Exit(1)
	}

	if !info.IsDir() {
		fmt.Printf("Error: '%s' is not a directory\n", *scanPath)
		os.Exit(1)
	}

	// Start the TUI
	if err := runTUI(*scanPath, *skipNetwork, *oneFilesystem); err != nil {
		fmt.Printf("Error running application: %v\n", err)
		os.Exit(1)
	}
}

func runTUI(rootPath string, skipNetwork bool, oneFilesystem bool) error {
	// Create the main model
	model := ui.NewModel(rootPath)

	// Create the Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Create cancellable context for scanner
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start scanning in the background
	go func() {
		progressChan := make(chan scanner.ScanProgress, 100)

		// Start progress update forwarder BEFORE scanning
		go func() {
			for progress := range progressChan {
				p.Send(ui.ScanProgressMsg(progress))
			}
		}()

		// Start the scan
		scn := scanner.NewScanner()
		scn.SetSkipNetwork(skipNetwork)
		scn.SetOneFilesystem(oneFilesystem)
		root, err := scn.Scan(ctx, rootPath, progressChan)

		// Send completion message
		p.Send(ui.ScanCompleteMsg{
			Root:           root,
			Err:            err,
			SkippedVolumes: scn.GetSkippedVolumes(),
		})
	}()

	// Run the program
	_, err := p.Run()

	// Cancel the scan when the program exits (user pressed 'q')
	cancel()

	return err
}

func printHelp() {
	fmt.Println(`SpaceForce - Disk Space Analyzer for macOS

A beautiful TUI application to help you find and clean up large files.

Usage:
  spaceforce [options]

Options:
  -path string
        Path to scan (default: current directory)
  -skip-network
        Skip network volumes and cloud storage during scan (default: true)
        Skips: network drives, iCloud Drive, Dropbox, Google Drive, etc.
        Use -skip-network=false to include these directories
  -one-filesystem
        Stay on one filesystem, don't cross mount points (default: true)
        Like 'du -x', prevents scanning external drives and mounted volumes
        Use -one-filesystem=false to scan across all mounted filesystems
  -version
        Show version information
  -help
        Show this help message

Controls:
  Tab         Switch between views
  1-5         Jump to specific view
  ↑/↓ or j/k  Navigate up/down
  Enter/Space Expand/collapse (in tree view)
  s           Change sort mode (in top list view)
  f           Toggle files (in top list view)
  d           Toggle directories (in top list view)
  q           Quit

Views:
  1. Tree View      - Hierarchical directory tree
  2. Top Items      - Largest files and folders sorted
  3. Breakdown      - File type statistics and breakdown
  4. Timeline       - Files grouped by modification date
  5. Errors         - Scan errors and warnings (permission denied, etc.)

Safety:
  SpaceForce uses intelligent safety checks to prevent deletion of:
  - System files and directories
  - Critical macOS components
  - Application bundles in /System
  - Configuration and credential files

  All deletions are moved to Trash and can be recovered.

Examples:
  # Scan current directory
  spaceforce

  # Scan your home directory
  spaceforce -path ~

  # Scan a specific directory
  spaceforce -path /Users/yourname/Downloads

For more information, visit: https://github.com/yourusername/spaceforce
`)
}
