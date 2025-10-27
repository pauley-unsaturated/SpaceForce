package safety

import (
	"os"
	"strings"
	"syscall"
)

// VolumeChecker detects network and special volumes
type VolumeChecker struct {
	skipNetwork bool
}

// NewVolumeChecker creates a new volume checker
func NewVolumeChecker(skipNetwork bool) *VolumeChecker {
	return &VolumeChecker{
		skipNetwork: skipNetwork,
	}
}

// ShouldSkipPath checks if a path should be skipped during scanning
func (vc *VolumeChecker) ShouldSkipPath(path string) (bool, string) {
	// Check for cloud-backed directories (iCloud, etc.)
	if vc.skipNetwork {
		if isCloud, reason := isCloudBackedPath(path); isCloud {
			return true, reason
		}
	}

	// Check if it's a network volume
	if vc.skipNetwork {
		isNetwork, fsType := vc.isNetworkVolume(path)
		if isNetwork {
			return true, "network volume (" + fsType + ")"
		}
	}

	return false, ""
}

// isCloudBackedPath checks if a path is cloud-backed (iCloud Drive, etc.)
func isCloudBackedPath(path string) (bool, string) {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false, ""
	}

	// iCloud Drive and synced app data
	if strings.HasPrefix(path, homeDir+"/Library/Mobile Documents") {
		return true, "iCloud synced directory"
	}

	// iCloud Desktop and Documents sync
	if strings.HasPrefix(path, homeDir+"/Library/Mobile Documents/com~apple~CloudDocs") {
		return true, "iCloud Drive"
	}

	// Other common cloud storage directories (user-installed)
	cloudDirs := []struct {
		path   string
		reason string
	}{
		{homeDir + "/Dropbox", "Dropbox"},
		{homeDir + "/Google Drive", "Google Drive"},
		{homeDir + "/OneDrive", "OneDrive"},
		{homeDir + "/Box", "Box"},
		{homeDir + "/Library/CloudStorage", "Cloud storage"},
	}

	for _, cloudDir := range cloudDirs {
		if strings.HasPrefix(path, cloudDir.path) {
			return true, cloudDir.reason
		}
	}

	return false, ""
}

// isNetworkVolume checks if a path is on a network filesystem
func (vc *VolumeChecker) isNetworkVolume(path string) (bool, string) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		// If we can't stat it, assume it's safe to try
		return false, ""
	}

	// Get filesystem type name from the statfs structure
	// On macOS, f_fstypename is a [16]int8 array
	fsTypeBytes := make([]byte, len(stat.Fstypename))
	for i, v := range stat.Fstypename {
		fsTypeBytes[i] = byte(v)
	}
	fsTypeName := string(fsTypeBytes)
	// Trim null bytes
	fsTypeName = strings.TrimRight(fsTypeName, "\x00")

	// Check for network filesystem types
	networkFSTypes := []string{
		"nfs",      // Network File System
		"smbfs",    // SMB/CIFS (Windows shares)
		"afpfs",    // Apple Filing Protocol
		"cifs",     // Common Internet File System
		"webdav",   // WebDAV
		"ftp",      // FTP mounts
		"davfs",    // DAV filesystem
		"mtpfs",    // MTP (Android devices)
	}

	fsTypeLower := strings.ToLower(fsTypeName)
	for _, netFS := range networkFSTypes {
		if fsTypeLower == netFS {
			return true, fsTypeName
		}
	}

	// Also check for common network mount points
	if strings.HasPrefix(path, "/net/") ||
		strings.HasPrefix(path, "/Network/") {
		return true, fsTypeName
	}

	// Check /Volumes for network shares (common on macOS)
	if strings.HasPrefix(path, "/Volumes/") {
		// /Volumes itself is fine, but check subdirectories
		if path != "/Volumes" && path != "/Volumes/" {
			// If it's a network FS type, skip it
			for _, netFS := range networkFSTypes {
				if fsTypeLower == netFS {
					return true, fsTypeName
				}
			}

			// Also check if it's marked as network in other ways
			// Some network volumes might show as "autofs" or other types
			if fsTypeLower == "autofs" {
				return true, fsTypeName
			}
		}
	}

	return false, ""
}

// GetLocalVolumes returns a list of local (non-network) volumes
func GetLocalVolumes() []VolumeInfo {
	volumes := make([]VolumeInfo, 0)

	// Common local mount points on macOS
	localPaths := []string{
		"/",
		"/System/Volumes/Data",
	}

	// Check /Volumes for local drives
	volumesDir := "/Volumes"
	if entries, err := os.ReadDir(volumesDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				localPaths = append(localPaths, volumesDir+"/"+entry.Name())
			}
		}
	}

	checker := NewVolumeChecker(true)
	for _, path := range localPaths {
		// Check if path exists
		if _, err := os.Stat(path); err != nil {
			continue
		}

		// Check if it's a network volume
		isNetwork, fsType := checker.isNetworkVolume(path)

		var stat syscall.Statfs_t
		size := int64(0)
		available := int64(0)
		if err := syscall.Statfs(path, &stat); err == nil {
			size = int64(stat.Blocks) * int64(stat.Bsize)
			available = int64(stat.Bavail) * int64(stat.Bsize)
			if fsType == "" {
				fsTypeBytes := make([]byte, len(stat.Fstypename))
				for i, v := range stat.Fstypename {
					fsTypeBytes[i] = byte(v)
				}
				fsType = string(fsTypeBytes)
				fsType = strings.TrimRight(fsType, "\x00")
			}
		}

		volumes = append(volumes, VolumeInfo{
			Path:      path,
			FSType:    fsType,
			IsNetwork: isNetwork,
			Size:      size,
			Available: available,
		})
	}

	return volumes
}

// VolumeInfo contains information about a volume
type VolumeInfo struct {
	Path      string
	FSType    string
	IsNetwork bool
	Size      int64
	Available int64
}
