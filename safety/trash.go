package safety

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// DeleteMethod represents different ways to delete files
type DeleteMethod int

const (
	DeleteToTrash DeleteMethod = iota // Move to Trash (safe, recoverable)
	DeletePermanent                     // Permanent deletion (unsafe)
)

// Deleter handles file deletion operations
type Deleter struct {
	method    DeleteMethod
	protector *Protector
}

// NewDeleter creates a new deleter
func NewDeleter(method DeleteMethod) *Deleter {
	return &Deleter{
		method:    method,
		protector: NewProtector(),
	}
}

// DeleteFile deletes a single file or directory
// Returns the size of the deleted item and any error
func (d *Deleter) DeleteFile(path string) (int64, error) {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("cannot stat file: %w", err)
	}

	// Safety check
	safe, reason := d.protector.IsSafeToDelete(path)
	if !safe {
		return 0, fmt.Errorf("file is protected: %s (%s)", path, reason)
	}

	size := info.Size()
	if info.IsDir() {
		// For directories, calculate total size
		size, _ = calculateDirSize(path)
	}

	switch d.method {
	case DeleteToTrash:
		err = d.moveToTrash(path)
	case DeletePermanent:
		err = os.RemoveAll(path)
	}

	if err != nil {
		return 0, err
	}

	return size, nil
}

// moveToTrash moves a file to the macOS Trash using osascript
func (d *Deleter) moveToTrash(path string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot get absolute path: %w", err)
	}

	// Use osascript to move to Trash (works on macOS)
	// This uses Finder's trash functionality which is the safest method
	script := fmt.Sprintf(`
		tell application "Finder"
			move POSIX file "%s" to trash
		end tell
	`, absPath)

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to move to trash: %w (output: %s)", err, string(output))
	}

	return nil
}

// calculateDirSize calculates the total size of a directory
func calculateDirSize(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}
