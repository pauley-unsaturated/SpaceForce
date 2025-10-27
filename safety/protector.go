package safety

import (
	"os"
	"path/filepath"
	"strings"
)

// Protector handles safety checks for file operations
type Protector struct {
	protectedPaths []string
	protectedExts  []string
}

// NewProtector creates a new protector with macOS default protections
func NewProtector() *Protector {
	return &Protector{
		protectedPaths: getProtectedPaths(),
		protectedExts:  getProtectedExtensions(),
	}
}

// IsSafeToDelete checks if a file/directory is safe to delete
func (p *Protector) IsSafeToDelete(path string) (bool, string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, "Cannot determine absolute path"
	}

	// Check if it's a protected system path
	for _, protectedPath := range p.protectedPaths {
		if strings.HasPrefix(absPath, protectedPath) {
			return false, "System path - critical for macOS operation"
		}
		if absPath == protectedPath {
			return false, "System path - critical for macOS operation"
		}
	}

	// Check if it's an application bundle
	if strings.HasSuffix(absPath, ".app") {
		// Apps in /Applications are generally safe to delete
		// But system apps in /System/Applications are not
		if strings.HasPrefix(absPath, "/System/Applications") {
			return false, "System application - built-in macOS app"
		}
		return true, "Application (can be reinstalled)"
	}

	// Check for protected extensions
	ext := filepath.Ext(absPath)
	for _, protectedExt := range p.protectedExts {
		if ext == protectedExt {
			return false, "Protected file type - may be system critical"
		}
	}

	// Check if it's in user directory
	homeDir, _ := os.UserHomeDir()

	// Protect the home directory itself
	if absPath == homeDir {
		return false, "Home directory - deleting this would destroy all user data"
	}

	if strings.HasPrefix(absPath, homeDir) {
		// User files are generally safe, but check for critical directories
		criticalUserDirs := []string{
			filepath.Join(homeDir, "Library"),           // Entire Library folder
			filepath.Join(homeDir, ".ssh"),              // SSH keys
			filepath.Join(homeDir, ".gnupg"),            // GPG keys
			filepath.Join(homeDir, ".aws"),              // AWS credentials
			filepath.Join(homeDir, ".config"),           // Config files
			filepath.Join(homeDir, ".kube"),             // Kubernetes config
			filepath.Join(homeDir, ".docker"),           // Docker config
		}

		for _, criticalDir := range criticalUserDirs {
			if absPath == criticalDir || strings.HasPrefix(absPath, criticalDir+"/") {
				return false, "Critical user data - may contain credentials, settings, or system config"
			}
		}

		return true, "User file - safe to delete"
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

	// For paths outside user directory, be conservative
	if strings.HasPrefix(absPath, "/usr/local") ||
	   strings.HasPrefix(absPath, "/opt") {
		return true, "Third-party software location - generally safe"
	}

	return false, "Unknown location - defaulting to safe mode"
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
