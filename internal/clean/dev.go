package clean

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lakshaymaurya-felt/purewin/internal/core"
	"github.com/lakshaymaurya-felt/purewin/pkg/whitelist"
)

// ─── Developer Cache Definitions ─────────────────────────────────────────────

// devCacheDef describes a developer tool cache location.
type devCacheDef struct {
	name        string
	paths       []string
	description string
}

// ─── Developer Cache Scanning ────────────────────────────────────────────────

// ScanDevCaches scans developer tool caches (npm, pip, Cargo, Gradle,
// NuGet, VS Code, JetBrains) and returns discovered items.
//
// SAFETY: .cargo\bin is NEVER scanned — only registry\cache and
// registry\src are included for Cargo.
func ScanDevCaches(wl *whitelist.Whitelist) []CleanItem {
	home := os.Getenv("USERPROFILE")
	local := os.Getenv("LOCALAPPDATA")
	roaming := os.Getenv("APPDATA")
	if home == "" || local == "" || roaming == "" {
		return nil
	}

	caches := []devCacheDef{
		{
			name:        "npm",
			paths:       []string{filepath.Join(roaming, "npm-cache")},
			description: "npm package cache",
		},
		{
			name: "pip",
			paths: []string{
				filepath.Join(local, "pip", "Cache"),
			},
			description: "Python pip cache",
		},
		{
			name: "Cargo",
			paths: []string{
				// NEVER include .cargo\bin — only registry caches.
				filepath.Join(home, ".cargo", "registry", "cache"),
				filepath.Join(home, ".cargo", "registry", "src"),
			},
			description: "Rust Cargo registry cache",
		},
		{
			name:        "Gradle",
			paths:       []string{filepath.Join(home, ".gradle", "caches")},
			description: "Gradle build cache",
		},
		{
			name:        "NuGet",
			paths:       []string{filepath.Join(home, ".nuget", "packages")},
			description: "NuGet package cache",
		},
		{
			name: "VS Code",
			paths: []string{
				filepath.Join(roaming, "Code", "Cache"),
				filepath.Join(roaming, "Code", "CachedData"),
			},
			description: "VS Code cache",
		},
	}

	var items []CleanItem

	for _, c := range caches {
		for _, p := range c.paths {
			if _, err := os.Stat(p); err != nil {
				continue
			}
			if wl != nil && wl.IsWhitelisted(p) {
				continue
			}
			dirItems := scanDirectory(p, "dev", c.description, wl)
			items = append(items, dirItems...)
		}
	}

	// JetBrains: only scan caches subdirectories within each IDE.
	jetbrainsItems := scanJetBrainsCaches(local, wl)
	items = append(items, jetbrainsItems...)

	return items
}

// ─── JetBrains Cache Scanning ────────────────────────────────────────────────

// scanJetBrainsCaches scans the "caches" directory within each JetBrains
// IDE installation directory, avoiding settings and other IDE data.
func scanJetBrainsCaches(local string, wl *whitelist.Whitelist) []CleanItem {
	jetbrainsDir := filepath.Join(local, "JetBrains")
	if _, err := os.Stat(jetbrainsDir); err != nil {
		return nil
	}

	entries, err := os.ReadDir(jetbrainsDir)
	if err != nil {
		return nil
	}

	var items []CleanItem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		cachesDir := filepath.Join(jetbrainsDir, e.Name(), "caches")
		if _, err := os.Stat(cachesDir); err != nil {
			continue
		}

		if wl != nil && wl.IsWhitelisted(cachesDir) {
			continue
		}

		desc := "JetBrains " + e.Name() + " cache"
		dirItems := scanDirectory(cachesDir, "dev", desc, wl)
		items = append(items, dirItems...)
	}

	return items
}

// ─── Go Module Cache ─────────────────────────────────────────────────────────

// GoModCacheSize returns the size of the Go module download cache.
// Returns 0 if Go is not installed or the cache doesn't exist.
func GoModCacheSize() int64 {
	cacheDir := goModCachePath()
	if cacheDir == "" {
		return 0
	}

	size, err := core.GetDirSize(cacheDir)
	if err != nil {
		return 0
	}
	return size
}

// CleanGoModCache runs `go clean -modcache` to remove the Go module cache.
// Returns the size that was freed. In dryRun mode, returns the cache size
// without deleting. Returns (0, nil) if Go is not installed.
func CleanGoModCache(dryRun bool) (int64, error) {
	if _, err := exec.LookPath("go"); err != nil {
		return 0, nil // Go not installed, skip silently.
	}

	cacheDir := goModCachePath()
	if cacheDir == "" {
		return 0, nil // No cache directory found, skip silently.
	}

	size, _ := core.GetDirSize(cacheDir)

	if dryRun {
		return size, nil
	}

	cmd := exec.Command("go", "clean", "-modcache")
	if output, err := cmd.CombinedOutput(); err != nil {
		return 0, fmt.Errorf("go clean -modcache failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}

	return size, nil
}

// goModCachePath returns the Go module cache directory path.
func goModCachePath() string {
	// First, try running `go env GOMODCACHE` to get the actual cache location.
	if _, err := exec.LookPath("go"); err == nil {
		cmd := exec.Command("go", "env", "GOMODCACHE")
		output, err := cmd.Output()
		if err == nil {
			modCache := strings.TrimSpace(string(output))
			if modCache != "" {
				if _, err := os.Stat(modCache); err == nil {
					return modCache
				}
			}
		}
	}

	// Check GOMODCACHE environment variable.
	if modCache := os.Getenv("GOMODCACHE"); modCache != "" {
		if _, err := os.Stat(modCache); err == nil {
			return modCache
		}
	}

	// Fall back to GOPATH/pkg/mod/cache.
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(os.Getenv("USERPROFILE"), "go")
	}

	cacheDir := filepath.Join(gopath, "pkg", "mod", "cache")
	if _, err := os.Stat(cacheDir); err == nil {
		return cacheDir
	}

	return ""
}

// ─── Docker Build Cache ──────────────────────────────────────────────────────

// DockerBuildCacheSize returns the size of Docker build cache.
// Returns 0 if Docker is not installed or the command fails.
func DockerBuildCacheSize() int64 {
	if _, err := exec.LookPath("docker"); err != nil {
		return 0 // Docker not installed.
	}

	// Try to get build cache size via docker system df.
	cmd := exec.Command("docker", "system", "df", "--format", "{{.Type}}\t{{.Size}}")
	output, err := cmd.Output()
	if err != nil {
		return 0 // Docker command failed, return 0 gracefully.
	}

	// Parse output looking for "Build Cache" line.
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Build Cache") {
			// Format is "Build Cache\tX.YGB" or "Build Cache\tXMB"
			parts := strings.Split(line, "\t")
			if len(parts) >= 2 {
				sizeStr := strings.TrimSpace(parts[1])
				// Parse human-readable size (e.g., "1.5GB", "250MB")
				size := parseDockerSize(sizeStr)
				return size
			}
		}
	}

	return 0
}

// parseDockerSize converts Docker's human-readable size format to bytes.
// Examples: "1.5GB" -> 1610612736, "250MB" -> 262144000
func parseDockerSize(sizeStr string) int64 {
	sizeStr = strings.TrimSpace(sizeStr)
	if sizeStr == "" || sizeStr == "0B" {
		return 0
	}

	var multiplier int64 = 1
	if strings.HasSuffix(sizeStr, "GB") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "GB")
	} else if strings.HasSuffix(sizeStr, "MB") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "MB")
	} else if strings.HasSuffix(sizeStr, "KB") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "KB")
	} else if strings.HasSuffix(sizeStr, "B") {
		sizeStr = strings.TrimSuffix(sizeStr, "B")
	}

	// Parse the numeric part (may be float like "1.5")
	var value float64
	if _, err := fmt.Sscanf(sizeStr, "%f", &value); err != nil {
		return 0
	}

	return int64(value * float64(multiplier))
}

// CleanDockerBuildCache runs `docker builder prune -af` to remove the
// Docker build cache. Returns (0, nil) if Docker is not installed.
// The caller should confirm with the user before invoking this.
func CleanDockerBuildCache(dryRun bool) (int64, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return 0, nil // Docker not installed, skip silently.
	}

	if dryRun {
		// Return the actual cache size for dry-run.
		return DockerBuildCacheSize(), nil
	}

	cmd := exec.Command("docker", "builder", "prune", "-af")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("docker builder prune failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}

	// Docker output includes "Total reclaimed space: X.YGB" but parsing
	// it reliably is fragile. Return 0 and let the output speak.
	return 0, nil
}

// IsDockerAvailable returns true if the docker CLI is on PATH.
func IsDockerAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// IsGoAvailable returns true if the go CLI is on PATH.
func IsGoAvailable() bool {
	_, err := exec.LookPath("go")
	return err == nil
}
