package safety

import (
	"os"
	"path/filepath"
)

// getAbsolutelyProtectedPaths returns paths that CANNOT be deleted under any circumstances
// These are critical system paths that would break macOS if deleted
func getAbsolutelyProtectedPaths() []string {
	return []string{
		// Core system directories - absolutely protected
		"/System",
		"/bin",
		"/sbin",
		"/usr/bin",
		"/usr/sbin",
		"/usr/lib",
		"/usr/libexec",
		"/private/etc",
		"/private/var/db",
		"/etc",
		"/dev",
		"/cores",
		"/Applications", // System Applications directory itself
		"/Users",        // Users directory itself (not contents)
		"/Library",      // System Library (everything under /Library)

		// Volumes (to prevent accidental deletion of mounted drives)
		"/Volumes/Macintosh HD",
		"/Volumes/Recovery",
	}
}

// getSensitivePaths returns paths that require explicit user confirmation to delete
// These are important user data/config locations but CAN be deleted if user confirms
func getSensitivePaths() []string {
	homeDir, _ := os.UserHomeDir()

	return []string{
		// Home directory itself (but not contents)
		homeDir,

		// User Library and important subdirectories - require confirmation
		filepath.Join(homeDir, "Library"),
		filepath.Join(homeDir, "Library/Application Support"),
		filepath.Join(homeDir, "Library/Preferences"),
		filepath.Join(homeDir, "Library/Containers"),
		filepath.Join(homeDir, "Library/Group Containers"),

		// Credential and configuration directories
		filepath.Join(homeDir, ".ssh"),
		filepath.Join(homeDir, ".gnupg"),
		filepath.Join(homeDir, ".aws"),
		filepath.Join(homeDir, ".config"),
		filepath.Join(homeDir, ".kube"),
		filepath.Join(homeDir, ".docker"),

		// Important user folders
		filepath.Join(homeDir, "Documents"),
		filepath.Join(homeDir, "Desktop"),
	}
}

// getProtectedPaths is deprecated - keeping for backward compatibility
// Use getAbsolutelyProtectedPaths instead
func getProtectedPaths() []string {
	return getAbsolutelyProtectedPaths()
}

// getProtectedExtensions returns file extensions that should be protected
func getProtectedExtensions() []string {
	return []string{
		// System files
		".kext",    // Kernel extensions
		".dylib",   // Dynamic libraries
		".framework", // Frameworks
		".bundle",  // System bundles
		".plugin",  // System plugins

		// Configuration
		".plist",   // Be careful with plists (many are critical)

		// Startup items
		".prefPane", // Preference panes
	}
}

// GetSafeCachePaths returns cache directories that are generally safe to clean
func GetSafeCachePaths() []string {
	return []string{
		"~/Library/Caches",                          // User caches
		"~/Library/Logs",                            // User logs
		"~/Library/Application Support/CrashReporter", // Crash reports
		"/Library/Caches",                           // System caches (be more careful)
		"~/Library/Safari/LocalStorage",             // Safari storage
		"~/Library/Safari/Databases",                // Safari databases
		"~/Library/Containers/*/Data/Library/Caches", // Sandboxed app caches
		"/System/Library/Caches",                    // System caches (only temp files)
		"/private/var/folders",                      // Temporary items
	}
}

// GetCommonBloatLocations returns locations where large, deletable files often accumulate
func GetCommonBloatLocations() []BloatLocation {
	return []BloatLocation{
		{
			Path:        "~/Library/Caches",
			Description: "Application caches",
			RiskLevel:   0,
			Reason:      "Apps will rebuild caches as needed",
		},
		{
			Path:        "~/Library/Developer/Xcode/DerivedData",
			Description: "Xcode build artifacts",
			RiskLevel:   0,
			Reason:      "Xcode will regenerate when building projects",
		},
		{
			Path:        "~/Library/Developer/Xcode/Archives",
			Description: "Xcode app archives",
			RiskLevel:   1,
			Reason:      "Old app archives (keep recent ones for distribution)",
		},
		{
			Path:        "~/Library/Developer/CoreSimulator",
			Description: "iOS Simulator data",
			RiskLevel:   0,
			Reason:      "Simulator will recreate runtime environments",
		},
		{
			Path:        "~/Library/Containers/com.docker.docker/Data",
			Description: "Docker images and containers",
			RiskLevel:   1,
			Reason:      "Can delete unused containers/images (use 'docker system prune')",
		},
		{
			Path:        "~/Library/Application Support/Steam",
			Description: "Steam game downloads",
			RiskLevel:   2,
			Reason:      "Game files can be re-downloaded",
		},
		{
			Path:        "~/.npm",
			Description: "NPM cache",
			RiskLevel:   0,
			Reason:      "NPM will re-download packages as needed",
		},
		{
			Path:        "~/.cache",
			Description: "Various application caches",
			RiskLevel:   0,
			Reason:      "Generic cache directory",
		},
		{
			Path:        "~/Library/Logs",
			Description: "Application logs",
			RiskLevel:   0,
			Reason:      "Old logs can usually be deleted",
		},
		{
			Path:        "~/Downloads",
			Description: "Downloaded files",
			RiskLevel:   1,
			Reason:      "Review before deleting - may contain important files",
		},
		{
			Path:        "/Library/Caches/Homebrew",
			Description: "Homebrew package cache",
			RiskLevel:   0,
			Reason:      "Brew can re-download packages (use 'brew cleanup')",
		},
		{
			Path:        "~/.cargo/registry",
			Description: "Rust cargo cache",
			RiskLevel:   0,
			Reason:      "Cargo will re-download crates as needed",
		},
		{
			Path:        "~/Library/Application Support/Code/CachedData",
			Description: "VS Code cache",
			RiskLevel:   0,
			Reason:      "VS Code will rebuild cache",
		},
		{
			Path:        "~/Library/Application Support/Google/Chrome/Default/Cache",
			Description: "Chrome browser cache",
			RiskLevel:   0,
			Reason:      "Browser will rebuild cache",
		},
		{
			Path:        "~/Library/Mail/V*/MailData/Envelope Index",
			Description: "Apple Mail cache",
			RiskLevel:   1,
			Reason:      "Mail will rebuild index (may take time)",
		},
		{
			Path:        "~/Library/Group Containers/*/Library/Caches",
			Description: "Shared app group caches",
			RiskLevel:   0,
			Reason:      "Apps will rebuild caches",
		},
	}
}

// BloatLocation represents a common location where space can be recovered
type BloatLocation struct {
	Path        string
	Description string
	RiskLevel   int // 0=safe, 1=low risk, 2=review carefully
	Reason      string
}
