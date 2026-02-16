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
	// Check GOMODCACHE first, then GOPATH/pkg/mod/cache, then default.
	if modCache := os.Getenv("GOMODCACHE"); modCache != "" {
		if _, err := os.Stat(modCache); err == nil {
			return modCache
		}
	}

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

// CleanDockerBuildCache runs `docker builder prune -af` to remove the
// Docker build cache. Returns (0, nil) if Docker is not installed.
// The caller should confirm with the user before invoking this.
func CleanDockerBuildCache(dryRun bool) (int64, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return 0, nil // Docker not installed, skip silently.
	}

	if dryRun {
		// Docker doesn't provide a simple way to query build cache size.
		// Return 0 for dry-run; the user will see "Docker build cache" as a line item.
		return 0, nil
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
