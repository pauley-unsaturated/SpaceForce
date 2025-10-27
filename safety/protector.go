package safety

import (
	"os"
	"path/filepath"
	"strings"
)

// Protector handles safety checks for file operations
type Protector struct {
	absolutelyProtectedPaths []string
	sensitivePaths           []string
	protectedExts            []string
}

// NewProtector creates a new protector with macOS default protections
func NewProtector() *Protector {
	return &Protector{
		absolutelyProtectedPaths: getAbsolutelyProtectedPaths(),
		sensitivePaths:           getSensitivePaths(),
		protectedExts:            getProtectedExtensions(),
	}
}

// IsSafeToDelete checks if a file/directory is safe to delete
// This only blocks ABSOLUTELY PROTECTED paths (system files)
// For sensitive paths that require confirmation, use RequiresConfirmation
func (p *Protector) IsSafeToDelete(path string) (bool, string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, "Cannot determine absolute path"
	}

	// Check if it's an absolutely protected system path
	for _, protectedPath := range p.absolutelyProtectedPaths {
		// Exact match or everything under it
		if absPath == protectedPath || strings.HasPrefix(absPath, protectedPath+"/") {
			return false, "System path - critical for macOS operation"
		}
	}

	// Check if it's an application bundle in /System
	if strings.HasSuffix(absPath, ".app") {
		// System apps in /System/Applications cannot be deleted
		if strings.HasPrefix(absPath, "/System/Applications") || strings.HasPrefix(absPath, "/System/Library") {
			return false, "System application - built-in macOS app"
		}
		// User/third-party apps are OK
		return true, "Application"
	}

	// Check for protected extensions (system libraries, kernel extensions)
	ext := filepath.Ext(absPath)
	for _, protectedExt := range p.protectedExts {
		if ext == protectedExt {
			// Only protect these extensions if they're in system locations
			if strings.HasPrefix(absPath, "/System") ||
			   strings.HasPrefix(absPath, "/Library") ||
			   strings.HasPrefix(absPath, "/usr") {
				return false, "System file type - critical for macOS"
			}
		}
	}

	// Check if path exists and is writable
	info, err := os.Stat(absPath)
	if err != nil {
		return false, "File does not exist or cannot be accessed"
	}

	// Check write permissions
	if info.Mode().Perm()&0200 == 0 {
		return false, "Read-only file - may be protected"
	}

	// Everything else is safe to delete (though may require confirmation)
	homeDir, _ := os.UserHomeDir()
	if strings.HasPrefix(absPath, homeDir) {
		return true, "User file"
	}

	// Third-party software locations
	if strings.HasPrefix(absPath, "/usr/local") || strings.HasPrefix(absPath, "/opt") {
		return true, "Third-party software"
	}

	// Conservative for unknown locations
	return false, "Unknown location - defaulting to protected"
}

// RequiresConfirmation checks if deleting a path requires extra user confirmation
// These are sensitive areas like ~/Library, credentials, etc.
func (p *Protector) RequiresConfirmation(path string) (bool, string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, ""
	}

	// Check if it's in a sensitive path
	for _, sensitivePath := range p.sensitivePaths {
		// Exact match
		if absPath == sensitivePath {
			return true, "This is a critical user directory"
		}
		// Anything under a sensitive path also requires confirmation
		if strings.HasPrefix(absPath, sensitivePath+"/") {
			// But get more specific reason based on the path
			if strings.Contains(absPath, "Library/Application Support") {
				return true, "Application data - may contain important settings or data"
			}
			if strings.Contains(absPath, "Library/Preferences") {
				return true, "Application preferences - may contain important settings"
			}
			if strings.Contains(absPath, "Library/Containers") || strings.Contains(absPath, "Library/Group Containers") {
				return true, "Sandboxed app data - may contain important app data"
			}
			if strings.Contains(absPath, ".ssh") {
				return true, "SSH keys and configuration - critical for authentication"
			}
			if strings.Contains(absPath, ".gnupg") {
				return true, "GPG keys - critical for encryption and signing"
			}
			if strings.Contains(absPath, ".aws") || strings.Contains(absPath, ".kube") {
				return true, "Cloud/cluster credentials - critical for infrastructure access"
			}
			if strings.Contains(absPath, "Documents") {
				return true, "Personal documents directory"
			}
			if strings.Contains(absPath, "Desktop") {
				return true, "Desktop items - may contain active work"
			}
			return true, "Sensitive user directory"
		}
	}

	return false, ""
}

// GetRiskLevel returns a risk level for deleting a path (0-3)
// 0 = safe, 1 = low risk, 2 = medium risk, 3 = high risk/protected
func (p *Protector) GetRiskLevel(path string) int {
	safe, reason := p.IsSafeToDelete(path)

	if !safe {
		if strings.Contains(reason, "System") || strings.Contains(reason, "critical") {
			return 3 // High risk
		}
		return 2 // Medium risk
	}

	homeDir, _ := os.UserHomeDir()
	absPath, _ := filepath.Abs(path)

	// Documents, Desktop, etc. are low risk (user knows what's there)
	userContentDirs := []string{
		filepath.Join(homeDir, "Documents"),
		filepath.Join(homeDir, "Desktop"),
		filepath.Join(homeDir, "Downloads"),
	}

	for _, dir := range userContentDirs {
		if strings.HasPrefix(absPath, dir) {
			return 1 // Low risk
		}
	}

	// Everything else that's safe is no risk
	return 0
}

// IsCache checks if a path is a cache directory
func (p *Protector) IsCache(path string) bool {
	absPath, _ := filepath.Abs(path)
	cachePaths := []string{
		"/Library/Caches",
		"Library/Caches",
		".cache",
		"Cache",
		"caches",
	}

	for _, cache := range cachePaths {
		if strings.Contains(absPath, cache) {
			return true
		}
	}

	return false
}

// IsLogFile checks if a file is a log file
func (p *Protector) IsLogFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	name := strings.ToLower(filepath.Base(path))

	return ext == ".log" ||
	       strings.Contains(name, ".log.") ||
	       strings.HasSuffix(name, ".log")
}
