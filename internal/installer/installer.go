package installer

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lakshaymaurya-felt/purewin/internal/core"
	"golang.org/x/sys/windows"
)

// InstallerFile represents a detected installer or archive file.
type InstallerFile struct {
	Path      string    // Full path to the file
	Name      string    // File name only
	Size      int64     // Size in bytes
	Extension string    // File extension (.exe, .msi, etc.)
	Source    string    // Source location (Downloads, Desktop, etc.)
	ModTime   time.Time // Last modification time
}

// scanLocation represents a directory to scan for installer files.
type scanLocation struct {
	Path        string // Directory path
	SourceLabel string // User-facing label
}

// GetScanLocations returns all locations to scan for installer files.
func GetScanLocations() []scanLocation {
	userProfile := os.Getenv("USERPROFILE")
	localAppData := os.Getenv("LOCALAPPDATA")
	temp := os.Getenv("TEMP")

	locations := []scanLocation{
		{Path: filepath.Join(userProfile, "Downloads"), SourceLabel: "Downloads"},
		{Path: filepath.Join(userProfile, "Desktop"), SourceLabel: "Desktop"},
		{Path: temp, SourceLabel: "Temp"},
	}

	// Chocolatey cache
	chocoCache := `C:\ProgramData\chocolatey\lib`
	if _, err := os.Stat(chocoCache); err == nil {
		locations = append(locations, scanLocation{
			Path:        chocoCache,
			SourceLabel: "Chocolatey",
		})
	}

	// Scoop cache
	scoopCache := filepath.Join(userProfile, "scoop", "cache")
	if _, err := os.Stat(scoopCache); err == nil {
		locations = append(locations, scanLocation{
			Path:        scoopCache,
			SourceLabel: "Scoop",
		})
	}

	// Winget cache (check for Microsoft.DesktopAppInstaller packages)
	wingetBase := filepath.Join(localAppData, "Packages")
	if entries, err := os.ReadDir(wingetBase); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if strings.Contains(entry.Name(), "Microsoft.DesktopAppInstaller") {
				cachePath := filepath.Join(wingetBase, entry.Name(), "LocalState")
				if _, err := os.Stat(cachePath); err == nil {
					locations = append(locations, scanLocation{
						Path:        cachePath,
						SourceLabel: "Winget",
					})
				}
			}
		}
	}

	return locations
}

// ScanInstallers scans for installer files matching the criteria.
// minAge is in days (0 = no age filter)
// minSize is in bytes (0 = no size filter)
func ScanInstallers(minAge int, minSize int64) ([]InstallerFile, error) {
	locations := GetScanLocations()
	var files []InstallerFile

	cutoffTime := time.Time{}
	if minAge > 0 {
		cutoffTime = time.Now().Add(-time.Duration(minAge) * 24 * time.Hour)
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc.Path); os.IsNotExist(err) {
			continue
		}

		err := scanLocationForInstallers(loc.Path, loc.SourceLabel, minSize, cutoffTime, &files)
		if err != nil {
			// Non-fatal: continue scanning other locations
			continue
		}
	}

	return files, nil
}

// scanLocationForInstallers scans a single location for installer files.
func scanLocationForInstallers(path, sourceLabel string, minSize int64, cutoffTime time.Time, files *[]InstallerFile) error {
	// For Chocolatey, look for .cache subdirectories
	if sourceLabel == "Chocolatey" {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			cachePath := filepath.Join(path, entry.Name(), ".cache")
			if _, err := os.Stat(cachePath); err == nil {
				_ = scanDirectoryForInstallers(cachePath, sourceLabel, minSize, cutoffTime, files)
			}
		}
		return nil
	}

	// For other locations, scan directly
	return scanDirectoryForInstallers(path, sourceLabel, minSize, cutoffTime, files)
}

// scanDirectoryForInstallers scans a directory (non-recursively) for installer files.
func scanDirectoryForInstallers(path, sourceLabel string, minSize int64, cutoffTime time.Time, files *[]InstallerFile) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Apply size filter
		if minSize > 0 && info.Size() < minSize {
			continue
		}

		// Apply age filter
		if !cutoffTime.IsZero() && info.ModTime().After(cutoffTime) {
			continue
		}

		// Check if file matches our criteria
		fullPath := filepath.Join(path, entry.Name())
		ext := strings.ToLower(filepath.Ext(entry.Name()))

		isInstaller := false
		switch ext {
		case ".exe", ".msi", ".msix", ".appx", ".appxbundle", ".msixbundle":
			isInstaller = true
		case ".zip", ".7z", ".rar":
			// Only include archives if they're large (>50MB)
			if info.Size() > 50*1024*1024 {
				isInstaller = true
			}
		}

		if !isInstaller {
			continue
		}

		// Check if file is locked (currently running)
		if isFileLocked(fullPath) {
			continue
		}

		file := InstallerFile{
			Path:      fullPath,
			Name:      entry.Name(),
			Size:      info.Size(),
			Extension: ext,
			Source:    sourceLabel,
			ModTime:   info.ModTime(),
		}

		*files = append(*files, file)
	}

	return nil
}

// isFileLocked checks if a file is currently in use (running executable).
func isFileLocked(path string) bool {
	// Try to open the file with exclusive access
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}

	handle, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, // No sharing - exclusive access
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)

	if err != nil {
		// If we can't open it exclusively, it's likely locked
		return true
	}

	// Close the handle immediately
	_ = windows.CloseHandle(handle)
	return false
}

// CleanInstallers deletes the specified installer files.
// Returns total bytes freed, number of files deleted, and any error.
func CleanInstallers(files []InstallerFile, dryRun bool) (int64, int, error) {
	var totalBytes int64
	var totalCount int
	var lastErr error

	for _, file := range files {
		freed, err := core.SafeDelete(file.Path, dryRun)
		if err != nil {
			lastErr = err
			continue
		}
		totalBytes += freed
		totalCount++
	}

	return totalBytes, totalCount, lastErr
}

// GroupBySource groups installer files by their source location.
func GroupBySource(files []InstallerFile) map[string][]InstallerFile {
	groups := make(map[string][]InstallerFile)
	for _, file := range files {
		groups[file.Source] = append(groups[file.Source], file)
	}
	return groups
}

// GetTotalSize calculates the total size of all files.
func GetTotalSize(files []InstallerFile) int64 {
	var total int64
	for _, file := range files {
		total += file.Size
	}
	return total
}
