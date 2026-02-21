package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// GitHubAPIURL is the GitHub API endpoint for releases
	GitHubAPIURL = "https://api.github.com/repos/lakshaymaurya-felt/purewin/releases/latest"

	// UpdateCheckCacheFile stores the last update check result
	UpdateCheckCacheFile = "last_update_check.json"

	// UpdateCheckInterval is how often to check for updates (24 hours)
	UpdateCheckInterval = 24 * time.Hour
)

// ReleaseInfo holds information about a GitHub release.
type ReleaseInfo struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	Body        string  `json:"body"`
	URL         string  `json:"html_url"`
	PublishedAt string  `json:"published_at"`
	Assets      []Asset `json:"assets"`
}

// Asset represents a release asset (downloadable file).
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// UpdateCheckCache stores the last update check result.
type UpdateCheckCache struct {
	LastCheck     time.Time `json:"last_check"`
	LatestVersion string    `json:"latest_version"`
	DownloadURL   string    `json:"download_url"`
}

// CheckForUpdate checks GitHub for the latest release.
// Returns the latest version, download URL, and any error.
func CheckForUpdate(currentVersion string) (latestVersion string, downloadURL string, err error) {
	// Normalize version strings (remove 'v' prefix if present)
	currentVersion = strings.TrimPrefix(currentVersion, "v")

	// Make HTTP request to GitHub API
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(GitHubAPIURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	// Parse response
	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", fmt.Errorf("failed to parse release info: %w", err)
	}

	latestVersion = strings.TrimPrefix(release.TagName, "v")

	// Find the appropriate asset for this platform
	assetName := getAssetNameForPlatform()
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return "", "", fmt.Errorf("no asset found for platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return latestVersion, downloadURL, nil
}

// CheckForUpdateBackground performs a non-blocking update check and caches the result.
// This is meant to be called at startup to check for updates without blocking the user.
func CheckForUpdateBackground(currentVersion string, cacheDir string) {
	go func() {
		// Check if we need to perform a check
		cachePath := filepath.Join(cacheDir, UpdateCheckCacheFile)
		cache, err := loadUpdateCache(cachePath)
		if err == nil && time.Since(cache.LastCheck) < UpdateCheckInterval {
			// Recent check, skip
			return
		}

		// Perform the check
		latestVersion, downloadURL, err := CheckForUpdate(currentVersion)
		if err != nil {
			return
		}

		// Save to cache
		newCache := UpdateCheckCache{
			LastCheck:     time.Now(),
			LatestVersion: latestVersion,
			DownloadURL:   downloadURL,
		}
		_ = saveUpdateCache(cachePath, newCache)
	}()
}

// loadUpdateCache reads the cached update check result.
func loadUpdateCache(path string) (*UpdateCheckCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cache UpdateCheckCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return &cache, nil
}

// saveUpdateCache writes the update check result to cache.
func saveUpdateCache(path string, cache UpdateCheckCache) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// getAssetNameForPlatform returns the expected asset name for the current platform.
func getAssetNameForPlatform() string {
	// Expected format: purewin_windows_amd64.exe, purewin_windows_arm64.exe, etc.
	return fmt.Sprintf("purewin_%s_%s.exe", runtime.GOOS, runtime.GOARCH)
}

// DownloadUpdate downloads the update from the given URL to a temporary file.
// Returns the path to the downloaded file.
func DownloadUpdate(url string) (string, error) {
	// Create temp file
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "purewin_update.exe")

	// Download
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Write to file
	out, err := os.Create(tempFile)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write update: %w", err)
	}

	return tempFile, nil
}

// ApplyUpdate replaces the current binary with the downloaded update.
// On Windows, this uses the rename trick to handle the "can't delete running exe" issue.
func ApplyUpdate(tempPath string) error {
	// Get current executable path
	currentExePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Resolve symlinks
	currentExePath, err = filepath.EvalSymlinks(currentExePath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Rename current exe to .old
	oldPath := currentExePath + ".old"

	// Remove any existing .old file
	_ = os.Remove(oldPath)

	// Rename current to .old
	if err := os.Rename(currentExePath, oldPath); err != nil {
		return fmt.Errorf("failed to rename current executable: %w", err)
	}

	// Copy new binary to the original location
	if err := copyFile(tempPath, currentExePath); err != nil {
		// Try to restore the old binary
		_ = os.Rename(oldPath, currentExePath)
		return fmt.Errorf("failed to copy new executable: %w", err)
	}

	// Schedule deletion of .old file using PowerShell
	_ = scheduleFileDeletion(oldPath)

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return out.Close()
}

// CleanupOldBinary removes the .old file left from a previous update.
func CleanupOldBinary() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}

	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return
	}

	oldPath := exePath + ".old"
	_ = os.Remove(oldPath)
}

// SelfRemove removes the binary, config, and cache directories.
// Returns an error if removal fails.
func SelfRemove(configDir, cacheDir string) error {
	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Remove config directory
	if configDir != "" {
		if err := os.RemoveAll(configDir); err != nil {
			return fmt.Errorf("failed to remove config directory: %w", err)
		}
	}

	// Remove cache directory (if different from config)
	if cacheDir != "" && cacheDir != configDir {
		if err := os.RemoveAll(cacheDir); err != nil {
			return fmt.Errorf("failed to remove cache directory: %w", err)
		}
	}

	// Schedule binary deletion using PowerShell
	// We can't delete ourselves while running, so we spawn a process that waits
	// and then deletes the binary
	return scheduleFileDeletion(exePath)
}

// scheduleFileDeletion spawns a PowerShell process that waits and then deletes the file.
// This is more reliable than the cmd.exe ping trick.
func scheduleFileDeletion(filePath string) error {
	// Use PowerShell to wait and delete. Escape single quotes and use -LiteralPath
	// to prevent wildcard expansion on paths with special characters.
	escaped := strings.ReplaceAll(filePath, "'", "''")
	psCommand := fmt.Sprintf("Start-Sleep -Seconds 2; Remove-Item -LiteralPath '%s' -Force", escaped)
	cmd := exec.Command("powershell", "-WindowStyle", "Hidden", "-Command", psCommand)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Start the process in detached mode
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to schedule file deletion: %w", err)
	}

	// Don't wait for the process to finish
	return nil
}

// RemoveFromPath removes the PureWin install directory from the user's PATH environment variable.
func RemoveFromPath(exePath string) error {
	// Get the directory containing the executable
	exeDir := filepath.Dir(exePath)

	// PowerShell script to remove from PATH.
	// Null-guard prevents clobbering an empty User PATH.
	// TrimEnd('\') normalises trailing-backslash mismatches.
	// Single-quote escaping prevents injection.
	escaped := strings.ReplaceAll(exeDir, "'", "''")
	psScript := fmt.Sprintf(`
		$exeDir = '%s'
		$path = [Environment]::GetEnvironmentVariable('Path', 'User')
		if ($null -eq $path) { exit 0 }
		$pathParts = $path -split ';'
		$newPath = $pathParts | Where-Object {
			$_ -ne '' -and $_.TrimEnd('\') -ine $exeDir.TrimEnd('\')
		}
		$newPathString = $newPath -join ';'
		if ($newPathString -eq $path) { exit 0 }
		[Environment]::SetEnvironmentVariable('Path', $newPathString, 'User')
	`, escaped)

	cmd := exec.Command("powershell", "-Command", psScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove from PATH: %w (output: %s)", err, string(output))
	}

	return nil
}

// IsNewerVersion compares two version strings and returns true if newer > current.
// Versions should be in semver format (e.g., "1.2.3" or "v1.2.3").
func IsNewerVersion(current, newer string) bool {
	// Remove 'v' prefix if present
	current = strings.TrimPrefix(current, "v")
	newer = strings.TrimPrefix(newer, "v")

	// Split versions by '.'
	currentParts := strings.Split(current, ".")
	newerParts := strings.Split(newer, ".")

	// Compare each part as integers
	maxLen := len(currentParts)
	if len(newerParts) > maxLen {
		maxLen = len(newerParts)
	}

	for i := 0; i < maxLen; i++ {
		// Get current part (default to 0 if missing)
		currentVal := 0
		if i < len(currentParts) {
			// Try to parse as integer, ignore non-numeric parts
			fmt.Sscanf(currentParts[i], "%d", &currentVal)
		}

		// Get newer part (default to 0 if missing)
		newerVal := 0
		if i < len(newerParts) {
			// Try to parse as integer, ignore non-numeric parts
			fmt.Sscanf(newerParts[i], "%d", &newerVal)
		}

		// Compare this part
		if newerVal > currentVal {
			return true
		} else if newerVal < currentVal {
			return false
		}
		// If equal, continue to next part
	}

	// All parts are equal
	return false
}
