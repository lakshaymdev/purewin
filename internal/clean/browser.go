package clean

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lakshaymaurya-felt/purewin/pkg/whitelist"
)

// ─── Browser Definitions ─────────────────────────────────────────────────────

// browserDef describes a Chromium-based browser's cache locations.
type browserDef struct {
	name    string   // Human-readable browser name.
	base    string   // Base "User Data" directory.
	subdirs []string // Cache subdirectories within each profile.
}

// ─── Browser Cache Scanning ──────────────────────────────────────────────────

// ScanBrowserCaches auto-detects installed browsers and scans their cache
// directories across ALL profiles (Default, Profile 1, Profile 2, …).
//
// Only cache directories are touched — bookmarks, passwords, cookies,
// history, extensions, and settings are NEVER included.
func ScanBrowserCaches(wl *whitelist.Whitelist) []CleanItem {
	local := os.Getenv("LOCALAPPDATA")

	browsers := []browserDef{
		{
			name: "Chrome",
			base: filepath.Join(local, "Google", "Chrome", "User Data"),
			subdirs: []string{
				"Cache",
				"Code Cache",
				"GPUCache",
				filepath.Join("Service Worker", "CacheStorage"),
			},
		},
		{
			name: "Edge",
			base: filepath.Join(local, "Microsoft", "Edge", "User Data"),
			subdirs: []string{
				"Cache",
				"Code Cache",
				"GPUCache",
				filepath.Join("Service Worker", "CacheStorage"),
			},
		},
		{
			name: "Brave",
			base: filepath.Join(local, "BraveSoftware", "Brave-Browser", "User Data"),
			subdirs: []string{
				"Cache",
				"Code Cache",
				"GPUCache",
			},
		},
	}

	var items []CleanItem

	// Scan Chromium-based browsers.
	for _, b := range browsers {
		if _, err := os.Stat(b.base); err != nil {
			continue // Browser not installed.
		}

		profiles := discoverChromiumProfiles(b.base)
		for _, profile := range profiles {
			for _, subdir := range b.subdirs {
				cacheDir := filepath.Join(profile, subdir)
				if _, err := os.Stat(cacheDir); err != nil {
					continue
				}
				desc := b.name + " cache"
				dirItems := scanDirectory(cacheDir, "browser", desc, wl)
				items = append(items, dirItems...)
			}
		}
	}

	// Firefox uses a different profile structure.
	firefoxItems := scanFirefoxCaches(local, wl)
	items = append(items, firefoxItems...)

	return items
}

// ─── Profile Discovery ───────────────────────────────────────────────────────

// discoverChromiumProfiles returns all profile directories within a
// Chromium-based browser's User Data directory. Profiles are named
// "Default", "Profile 1", "Profile 2", etc.
func discoverChromiumProfiles(userDataDir string) []string {
	entries, err := os.ReadDir(userDataDir)
	if err != nil {
		return nil
	}

	var profiles []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Chromium profiles: "Default", "Profile 1", "Profile 2", …
		// Also "Guest Profile", "System Profile" may exist.
		if name == "Default" || strings.HasPrefix(name, "Profile ") {
			profiles = append(profiles, filepath.Join(userDataDir, name))
		}
	}

	return profiles
}

// ─── Firefox ─────────────────────────────────────────────────────────────────

// scanFirefoxCaches scans Firefox cache2 directories across all profiles.
// Only the cache2 directory is scanned — profile data (bookmarks,
// passwords, extensions) is never touched.
func scanFirefoxCaches(local string, wl *whitelist.Whitelist) []CleanItem {
	profilesDir := filepath.Join(local, "Mozilla", "Firefox", "Profiles")
	if _, err := os.Stat(profilesDir); err != nil {
		return nil
	}

	var items []CleanItem

	profiles, _ := filepath.Glob(filepath.Join(profilesDir, "*"))
	for _, profile := range profiles {
		info, err := os.Stat(profile)
		if err != nil || !info.IsDir() {
			continue
		}

		cacheDir := filepath.Join(profile, "cache2")
		if _, err := os.Stat(cacheDir); err != nil {
			continue
		}

		dirItems := scanDirectory(cacheDir, "browser", "Firefox cache", wl)
		items = append(items, dirItems...)
	}

	return items
}
